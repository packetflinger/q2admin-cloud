package backend

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/database"
	"github.com/packetflinger/q2admind/frontend"
	pb "github.com/packetflinger/q2admind/proto"
)

// groupTestSetup points the backend at a fresh database and two frontends
// owned by owner (one connected) plus one owned by someone else.
func groupTestSetup(t *testing.T, owner string) (up, down, theirs *frontend.Frontend) {
	t.Helper()
	d, err := database.Open(filepath.Join(t.TempDir(), "q2a.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	savedDB, savedFEs, savedConfig := db, be.frontends, be.config
	t.Cleanup(func() { d.Handle.Close(); db, be.frontends, be.config = savedDB, savedFEs, savedConfig })
	db = d
	be.config = &pb.Config{WebRoot: "../api/website"}
	up = connectedFrontend(t, owner)
	up.ID = 1
	down = &frontend.Frontend{ID: 2, Name: "fe2", UUID: "fe2-uuid", Owner: owner}
	theirs = &frontend.Frontend{ID: 3, Name: "theirs", UUID: "theirs-uuid", Owner: "other@example.com"}
	be.frontends = []*frontend.Frontend{up, down, theirs}
	return up, down, theirs
}

func TestGroups(t *testing.T) {
	owner := "admin@example.com"
	up, down, theirs := groupTestSetup(t, owner)

	if err := CreateGroup(owner, "Bad Name"); err == nil {
		t.Error("invalid group name accepted")
	}
	if err := CreateGroup(owner, "east"); err != nil {
		t.Fatal(err)
	}
	if err := CreateGroup(owner, "east"); err == nil {
		t.Error("duplicate group name accepted")
	}
	if err := CreateGroup("other@example.com", "east"); err != nil {
		t.Errorf("names should only be unique per owner: %v", err)
	}
	CreateGroup(owner, "ctf")
	for _, step := range []struct {
		group string
		fe    *frontend.Frontend
		add   bool
	}{{"east", up, true}, {"east", down, true}, {"ctf", up, true}, {"east", up, true}} { // re-adding is harmless
		if err := SetGroupMember(owner, step.group, step.fe, step.add); err != nil {
			t.Fatal(err)
		}
	}
	if err := SetGroupMember(owner, "east", theirs, true); err == nil {
		t.Error("added someone else's server")
	}

	groups, err := MyGroups(owner)
	if err != nil || len(groups) != 2 || groups[0].Name != "ctf" || len(groups[1].Members) != 2 {
		t.Fatalf("MyGroups = %+v, %v", groups, err)
	}

	sent, offline, err := SendGroupCommand(owner, "east", "say hi", owner)
	if err != nil || sent != 1 || offline != 1 {
		t.Errorf("SendGroupCommand = %d sent, %d offline, %v", sent, offline, err)
	}
	if _, _, err := SendGroupCommand(owner, "east", "say hi\nquit", owner); err == nil {
		t.Error("multi-line group command accepted")
	}

	SetGroupMember(owner, "east", down, false)
	if g, _ := MyGroup(owner, "east"); len(g.Members) != 1 || g.Members[0] != up {
		t.Errorf("remove failed: %+v", g.Members)
	}
	if err := DeleteGroup(owner, "east"); err != nil {
		t.Fatal(err)
	}
	if groups, _ := MyGroups(owner); len(groups) != 1 {
		t.Errorf("after delete: %d groups", len(groups))
	}
	if err := DeleteGroup("other@example.com", "ctf"); err == nil {
		t.Error("deleted someone else's group")
	}
}

func TestSSHGroupCommands(t *testing.T) {
	owner := "admin@example.com"
	groupTestSetup(t, owner)
	run := func(line string) string {
		c, _ := ParseCmdArgs(line)
		return sshGroupCommand(owner, c)
	}
	for _, step := range []struct{ line, want string }{
		{"groups", "don't have any groups"},
		{"group create east", "ok"},
		{"group add east fe1", "ok"},
		{"group add east theirs", "don't own"},
		{"group add east nope", "don't own"},
		{"group show east", "fe1"},
		{"groups", "east"},
		{"group cmd east say hello there", "sent to 1 server(s), 0 offline"},
		{"group remove east fe1", "ok"},
		{"group bogus", "Usage"},
		{"group add east", "Usage"},
		{"group delete east", "ok"},
	} {
		if got := run(step.line); !strings.Contains(got, step.want) {
			t.Errorf("%q printed %q, want it to contain %q", step.line, got, step.want)
		}
	}
}

func TestGroupPages(t *testing.T) {
	user := &pb.User{Email: "admin@example.com"}
	up, _, _ := groupTestSetup(t, user.Email)
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, user)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	post := func(path string, form url.Values) *http.Response {
		t.Helper()
		res, err := client.PostForm(srv.URL+path, form)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	status := func(path string) int {
		t.Helper()
		res, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	if res := post("/groups", url.Values{"name": {"east"}}); res.Header.Get("Location") != "/groups/east?notice=groupcreated" {
		t.Fatalf("create redirected to %q", res.Header.Get("Location"))
	}
	post("/groups/east/action", url.Values{"action": {"add"}, "server": {up.UUID}})
	if g, _ := MyGroup(user.Email, "east"); len(g.Members) != 1 {
		t.Error("server not added")
	}
	if res := post("/groups/east/action", url.Values{"action": {"add"}, "server": {"theirs-uuid"}}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("adding someone else's server: status %d", res.StatusCode)
	}
	if res := post("/groups/east/action", url.Values{"action": {"command"}, "cmd": {"say hi"}}); !strings.Contains(res.Header.Get("Location"), "sent=1&offline=0") {
		t.Errorf("command redirected to %q", res.Header.Get("Location"))
	}
	for _, p := range []string{"/groups", "/groups/east", "/groups/east?sent=1&offline=0"} {
		if code := status(p); code != 200 {
			t.Errorf("%s: status %d", p, code)
		}
	}
	if code := status("/groups/nope"); code != http.StatusNotFound {
		t.Errorf("unknown group: status %d", code)
	}
	post("/groups/east/action", url.Values{"action": {"delete"}})
	if groups, _ := MyGroups(user.Email); len(groups) != 0 {
		t.Error("group not deleted")
	}
}

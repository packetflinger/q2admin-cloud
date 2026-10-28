package backend

import (
	"io"
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
	createRuleTable(t, &db) // group rules live here
	be.config = &pb.Config{}
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
		{"group add east theirs", "don't have"},
		{"group add east nope", "don't have"},
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

func TestGroupRules(t *testing.T) {
	user := &pb.User{Email: "admin@example.com"}
	up, down, _ := groupTestSetup(t, user.Email)
	for _, fe := range []*frontend.Frontend{up, down} {
		fe.Data = &db
	}
	db.Handle.Exec(`CREATE TABLE frontend (id INTEGER, uuid TEXT, name TEXT);
		CREATE TABLE player_stat (id INTEGER PRIMARY KEY, player INTEGER, frags INTEGER, deaths INTEGER, suicides INTEGER, kdr INTEGER, play_time INTEGER)`)
	CreateGroup(user.Email, "east")
	SetGroupMember(user.Email, "east", up, true)
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, user)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	post := func(path string, form url.Values) string {
		t.Helper()
		res, err := client.PostForm(srv.URL+path, form)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.Header.Get("Location")
	}
	get := func(path string) (int, string) {
		t.Helper()
		res, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}

	loc := post("/groups/east/rules/add", url.Values{"type": {"ban"}, "description": {"group ban"}, "addresses": {"10.0.0.0/8"}, "expires": {"never"}})
	if !strings.HasPrefix(loc, "/rules/") {
		t.Fatalf("creating a group rule redirected to %q", loc)
	}
	ruleURL := strings.TrimSuffix(loc, "?notice=rulesaved")
	id := strings.TrimSuffix(strings.TrimPrefix(ruleURL, "/rules/"), "/view")

	applies := func(fe *frontend.Frontend) bool {
		rules, err := fe.FetchGroupRules()
		if err != nil {
			t.Fatal(err)
		}
		return len(rules) == 1 && rules[0].GetUuid() == id
	}
	if !applies(up) || applies(down) {
		t.Error("the group rule should apply to the group's server only")
	}
	if own, _ := up.FetchRules(); len(own) != 0 {
		t.Errorf("a group rule showed up as the server's own rule: %v", own)
	}
	if _, body := get("/sv/fe1-uuid/fe1/rules"); !strings.Contains(body, "From groups") || !strings.Contains(body, "group ban") {
		t.Error("the server's rule list should show the group rule")
	}
	if code, body := get(ruleURL); code != 200 || !strings.Contains(body, "every server in the group") {
		t.Errorf("group rule page: status %d", code)
	}
	post("/rules/"+id+"/edit", url.Values{"type": {"mute"}, "names": {"^x"}, "expires": {"never"}})
	if rules, _ := up.FetchGroupRules(); len(rules) != 1 || rules[0].GetType() != pb.RuleType_MUTE {
		t.Errorf("edit not saved: %v", rules)
	}

	SetGroupMember(user.Email, "east", up, false)
	if applies(up) {
		t.Error("the group rule still applies after the server left the group")
	}
	SetGroupMember(user.Email, "east", up, true)

	if loc := post("/rules/"+id+"/delete", nil); loc != "/groups/east?notice=ruledeleted#rules" {
		t.Errorf("deleting a group rule redirected to %q", loc)
	}
	if applies(up) {
		t.Error("deleted group rule still applies")
	}

	post("/groups/east/rules/add", url.Values{"type": {"ban"}, "addresses": {"10.0.0.0/8"}, "expires": {"never"}})
	DeleteGroup(user.Email, "east")
	var n int
	db.Handle.QueryRow("SELECT COUNT(*) FROM rule WHERE server_group != 0").Scan(&n)
	if n != 0 {
		t.Errorf("%d group rules left after deleting the group", n)
	}
}

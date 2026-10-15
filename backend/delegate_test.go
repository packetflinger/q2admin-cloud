package backend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"
	pb "github.com/packetflinger/q2admind/proto"
)

func TestDelegation(t *testing.T) {
	owner := &pb.User{Email: "owner@example.com"}
	delegate := &pb.User{Email: "Delegate@example.com"}
	stranger := &pb.User{Email: "stranger@example.com"}
	fe, _, _ := groupTestSetup(t, owner.Email) // fe1, connected, ID 1
	fe.Data = &db
	createRuleTable(t, &db)
	for _, q := range []string{ // what the server page's stats and search read
		`CREATE TABLE frontend (id INTEGER, uuid TEXT, name TEXT)`,
		`CREATE TABLE player_stat (id INTEGER PRIMARY KEY, player INTEGER, frags INTEGER, deaths INTEGER, suicides INTEGER, kdr INTEGER, play_time INTEGER)`,
	} {
		if _, err := db.Handle.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	fe.Players = []frontend.Player{{ClientID: 0, ConnectTime: 1, Name: "alpha", IP: "192.0.2.7"}}
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	ownerC := signedInClient(t, srv, owner)
	delegateC := signedInClient(t, srv, delegate) // also registers the delegate
	strangerC := signedInClient(t, srv, stranger)
	for _, c := range []*http.Client{ownerC, delegateC, strangerC} {
		c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	do := func(c *http.Client, method, path string, form url.Values) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header.Get("Location") + string(body)
	}
	base := "/sv/fe1-uuid/fe1"

	if _, loc := do(ownerC, "POST", base+"/delegates", url.Values{"action": {"add"}, "email": {"nobody@example.com"}}); !strings.Contains(loc, "notice=nouser") {
		t.Errorf("unregistered email: %s", loc)
	}
	if code, _ := do(strangerC, "POST", base+"/delegates", url.Values{"action": {"add"}, "email": {stranger.Email}}); code != http.StatusNotFound {
		t.Errorf("a stranger shared the server with themselves: %d", code)
	}
	if _, loc := do(ownerC, "POST", base+"/delegates", url.Values{"action": {"add"}, "email": {"delegate@EXAMPLE.com"}}); !strings.Contains(loc, "notice=delegated") {
		t.Fatalf("share: %s", loc)
	}
	if got := FrontendsByIdentity(delegate.Email); len(got) != 1 || got[0] != fe {
		t.Error("shared server missing from the delegate's list")
	}

	// what a delegate can do
	for _, step := range []struct {
		method, path string
		form         url.Values
		want         int
	}{
		{"GET", base, nil, 200},
		{"GET", base + "/player/0", nil, 200},
		{"GET", base + "/rules", nil, 200},
		{"GET", base + "/console", nil, 200},
		{"POST", base + "/console/command", url.Values{"cmd": {"status"}}, http.StatusNoContent},
		{"POST", base + "/map", url.Values{"map": {"q2dm1"}}, http.StatusSeeOther},
		{"POST", base + "/player/0/action", url.Values{"action": {"message"}, "msg": {"hi"}}, http.StatusSeeOther},
		{"POST", base + "/rules/add", url.Values{"type": {"ban"}, "addresses": {"10.0.0.0/8"}, "expires": {"never"}}, http.StatusSeeOther},
		{"GET", "/search/fe1-uuid?q=x", nil, 200},
		// and what they can't
		{"POST", base + "/edit", url.Values{"uuid": {"fe1-uuid"}, "servername": {"x"}, "serveraddr": {"1.2.3.4"}}, http.StatusForbidden},
		{"POST", base + "/delete", nil, http.StatusForbidden},
		{"POST", base + "/change-uuid", nil, http.StatusForbidden},
		{"GET", base + "/manage-keys", nil, http.StatusForbidden},
		{"POST", base + "/manage-keys", url.Values{"action": {"SaveKey"}, "keydata": {"x"}}, http.StatusForbidden},
		{"POST", base + "/delegates", url.Values{"action": {"add"}, "email": {stranger.Email}}, http.StatusForbidden},
	} {
		if code, _ := do(delegateC, step.method, step.path, step.form); code != step.want {
			t.Errorf("delegate %s %s: status %d, want %d", step.method, step.path, code, step.want)
		}
	}
	if _, body := do(delegateC, "GET", base, nil); strings.Contains(body, `id="settings"`) || !strings.Contains(body, ">shared</span>") {
		t.Error("delegate's server page should hide settings and say it's shared")
	}
	if _, body := do(ownerC, "GET", base, nil); !strings.Contains(body, "Delegate@example.com") {
		t.Error("owner's settings should list the delegate")
	}
	if code, _ := do(strangerC, "GET", base, nil); code != http.StatusNotFound {
		t.Errorf("stranger viewing the server: status %d, want 404", code)
	}

	do(ownerC, "POST", base+"/delegates", url.Values{"action": {"remove"}, "email": {"delegate@example.com"}})
	if code, _ := do(delegateC, "GET", base, nil); code != http.StatusNotFound {
		t.Errorf("removed delegate still has access: %d", code)
	}
	var n int
	db.Handle.QueryRow("SELECT COUNT(*) FROM frontend_delegate").Scan(&n)
	if n != 0 || len(fe.Delegates) != 0 {
		t.Errorf("delegate not removed: %d rows, %v", n, fe.Delegates)
	}
}

func TestLoadFrontendsReadsDelegates(t *testing.T) {
	groupTestSetup(t, "owner@example.com") // fresh database
	for _, q := range []string{
		`CREATE TABLE frontend (id INTEGER PRIMARY KEY, uuid TEXT, name TEXT, owner TEXT, enabled INTEGER DEFAULT 0,
			description TEXT DEFAULT '', allow_teleport INTEGER DEFAULT 0, allow_invite INTEGER DEFAULT 0, ip_address TEXT DEFAULT '',
			port INTEGER DEFAULT 27910, public_key_data TEXT DEFAULT '', verified INTEGER DEFAULT 0, invite_tokens INTEGER DEFAULT 3,
			invite_token_frequency INTEGER DEFAULT 300, delete_protection INTEGER DEFAULT 0)`,
		`CREATE TABLE rule (id INTEGER, frontend INTEGER, uuid TEXT, rule_type INTEGER, creation_time INTEGER, expiration_time INTEGER,
			delay INTEGER, stifle_length INTEGER, disabled INTEGER, scope TEXT, timespec_before TEXT, timespec_after TEXT, timespec_every TEXT,
			timespec_playtime TEXT, addresses TEXT, names TEXT, hostnames TEXT, clients TEXT, userinfo TEXT, message TEXT, description TEXT, vpn INTEGER)`,
		`INSERT INTO frontend (id, uuid, name, owner) VALUES (5, 'u5', 'five', 'owner@example.com'), (6, 'u6', 'six', 'owner@example.com')`,
		`INSERT INTO frontend_delegate VALUES (5, 'a@example.com'), (5, 'b@example.com')`,
	} {
		if _, err := db.Handle.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	fes, err := be.LoadFrontends()
	if err != nil {
		t.Fatal(err)
	}
	if len(fes) != 2 || strings.Join(fes[0].Delegates, ",") != "a@example.com,b@example.com" || len(fes[1].Delegates) != 0 {
		t.Errorf("delegates loaded as %v / %v", fes[0].Delegates, fes[1].Delegates)
	}
}

func TestShareGroup(t *testing.T) {
	owner := &pb.User{Email: "owner@example.com"}
	friend := &pb.User{Email: "friend@example.com"}
	up, down, theirs := groupTestSetup(t, owner.Email)
	theirs.Delegates = []string{owner.Email} // in owner's group, but not owner's to share
	down.Delegates = []string{friend.Email}  // already shared
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, owner)
	signedInClient(t, srv, friend) // registers friend
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	CreateGroup(owner.Email, "all")
	for _, fe := range []*frontend.Frontend{up, down, theirs} {
		if err := SetGroupMember(owner.Email, "all", fe, true); err != nil {
			t.Fatal(err)
		}
	}
	share := func(email string) string {
		t.Helper()
		res, err := client.PostForm(srv.URL+"/groups/all/action", url.Values{"action": {"share"}, "email": {email}})
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.Header.Get("Location")
	}
	if loc := share("nobody@example.com"); !strings.Contains(loc, "notice=nouser") {
		t.Errorf("unregistered email: %s", loc)
	}
	if loc := share("FRIEND@example.com"); !strings.Contains(loc, "shared=1&skipped=2") {
		t.Errorf("first share redirected to %s", loc)
	}
	if !slices.Contains(up.Delegates, friend.Email) || slices.Contains(theirs.Delegates, friend.Email) {
		t.Errorf("delegates: up %v, theirs %v", up.Delegates, theirs.Delegates)
	}
	if loc := share(friend.Email); !strings.Contains(loc, "shared=0&skipped=3") {
		t.Errorf("repeat share redirected to %s", loc)
	}
	var n int
	db.Handle.QueryRow("SELECT COUNT(*) FROM frontend_delegate WHERE email = ?", friend.Email).Scan(&n)
	if n != 1 { // down's existing share was only in memory in this test
		t.Errorf("%d delegate rows saved, want 1", n)
	}
}

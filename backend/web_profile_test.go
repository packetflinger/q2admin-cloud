package backend

import (
	"github.com/packetflinger/q2admind/database"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

func TestDisplayName(t *testing.T) {
	for _, tc := range []struct {
		user *pb.User
		want string
	}{
		{&pb.User{Email: "joe@example.com", Name: "claire"}, "claire"},
		{&pb.User{Email: "joe@example.com"}, "joe"},
		{&pb.User{Email: "no-at-sign"}, "no-at-sign"},
	} {
		if got := DisplayName(tc.user); got != tc.want {
			t.Errorf("DisplayName(%v) = %q, want %q", tc.user, got, tc.want)
		}
	}
}

func TestProfileNameAndConsoleEcho(t *testing.T) {
	user := &pb.User{Email: "joe@example.com"}
	fe, _, _ := groupTestSetup(t, user.Email) // fresh database
	if _, err := db.Handle.Exec(`CREATE TABLE user (id INTEGER PRIMARY KEY, email TEXT, name TEXT DEFAULT '');
		INSERT INTO user (email) VALUES ('joe@example.com')`); err != nil {
		t.Fatal(err)
	}
	watch := make(chan frontend.ConsoleLine, 8)
	fe.AddTerminal(&watch, 0)
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
	echo := func() string {
		t.Helper()
		post("/sv/fe1-uuid/fe1/console/command", url.Values{"cmd": {"status"}})
		return (<-watch).Text
	}

	if got := echo(); !strings.HasPrefix(got, "[joe] > status") {
		t.Errorf("with no name the echo is %q, want the email's username", got)
	}
	<-watch // the status report

	if res := post("/me", url.Values{"name": {"bad\nname"}}); res.StatusCode != 200 {
		t.Errorf("a name with a newline: status %d, want the form shown again", res.StatusCode)
	}
	if res := post("/me", url.Values{"name": {"  claire  "}}); res.Header.Get("Location") != "/me?notice=namesaved" {
		t.Fatalf("saving a name redirected to %q", res.Header.Get("Location"))
	}
	var saved string
	db.Handle.QueryRow("SELECT name FROM user WHERE email = ?", user.Email).Scan(&saved)
	if saved != "claire" || user.Name != "claire" {
		t.Errorf("name saved as %q, in memory %q", saved, user.Name)
	}
	if got := echo(); !strings.HasPrefix(got, "[claire] > status") {
		t.Errorf("with a name the echo is %q", got)
	}
}

func TestLoadUsersReadsName(t *testing.T) {
	file := filepath.Join(t.TempDir(), "q2a.sqlite")
	d, err := database.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	// the user table as it was before name
	_, err = d.Handle.Exec(`CREATE TABLE user (id INTEGER PRIMARY KEY, uuid TEXT, email TEXT, disabled INTEGER,
		allow_ssh INTEGER, ssh_key TEXT, allow_rpc INTEGER, rpc_key TEXT);
		INSERT INTO user (uuid, email, disabled, allow_ssh, ssh_key, allow_rpc, rpc_key) VALUES ('u', 'joe@example.com', 0, 1, '', 0, '')`)
	if err != nil {
		t.Fatal(err)
	}
	d.Handle.Close()
	savedDB := db
	defer func() { db = savedDB }()
	if db, err = database.Open(file); err != nil { // adds the column
		t.Fatal(err)
	}
	defer db.Handle.Close()
	db.Handle.Exec("UPDATE user SET name = 'claire'")
	users, err := be.LoadUsers()
	if err != nil || len(users) != 1 || users[0].GetName() != "claire" || !users[0].GetAllowSsh() {
		t.Errorf("LoadUsers = %v, %v", users, err)
	}
}

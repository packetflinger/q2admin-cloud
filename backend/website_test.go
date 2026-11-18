package backend

import (
	"bytes"
	"github.com/packetflinger/q2admind/database"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

func TestExecuteLocked(t *testing.T) {
	fe := &frontend.Frontend{Name: "fe1"}
	held := func() bool { // reports whether fe is locked right now
		if fe.Mu.TryLock() {
			fe.Mu.Unlock()
			return false
		}
		return true
	}
	tmpl := template.Must(template.New("t").Funcs(template.FuncMap{"held": held}).Parse(`{{define "page"}}{{.Name}} {{held}}{{end}}`))
	var out bytes.Buffer
	// fe passed twice must not self-deadlock
	if err := executeLocked(&out, tmpl, "page", fe, fe, fe); err != nil {
		t.Fatal(err)
	}
	if out.String() != "fe1 true" {
		t.Errorf("output = %q, want %q", out.String(), "fe1 true")
	}
	if held() {
		t.Error("frontend still locked after render")
	}
}

// renderPage executes a page the way its handler does, failing on any
// template error.
func renderPage(t *testing.T, file, name string, data PageResponse) string {
	t.Helper()
	saved := be.config
	be.config = &pb.Config{}
	defer func() { be.config = saved }()
	tmpl, err := parsePage(file)
	if err != nil {
		t.Fatalf("parsePage(%q): %v", file, err)
	}
	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, name, data); err != nil {
		t.Fatalf("executing %q: %v", name, err)
	}
	return out.String()
}

// samplePage is a signed-in page about one connected frontend with a player
// and a rule, enough to exercise every template branch.
func samplePage() PageResponse {
	conn, _ := net.Pipe()
	fe := &frontend.Frontend{
		Name:       "pf-tdm",
		UUID:       "83bfa705-7257-4843-b83a-9bc3ad2a3dfc",
		IPAddress:  "100.64.1.2",
		Port:       27910,
		Connected:  true,
		Trusted:    true,
		Connection: &conn,
		CurrentMap: "q2dm1",
		MaxPlayers: 2,
		ServerVars: map[string]string{"dmflags": "1040", "hostname": "test"},
		Rules: []*pb.Rule{{
			Uuid:        "5d435f38-7756-4899-b43f-b44599f442c5",
			Type:        pb.RuleType_BAN,
			Address:     []string{"10.0.0.0/8"},
			Description: []string{"test ban"},
		}},
	}
	fe.Players = []frontend.Player{
		{ClientID: 0, ConnectTime: 100, Name: "alpha", IP: "192.0.2.1", Frontend: fe},
		{ClientID: 1},
	}
	fe.PlayerCount = 1
	data := PageResponse{
		SessionUser: &pb.User{Email: "admin@example.com"},
		Frontends:   []*frontend.Frontend{fe},
		Frontend:    fe,
		Player:      &fe.Players[0],
		Rule:        fe.Rules[0],
		Rules:       fe.Rules,
		ServerNames: map[string]string{fe.UUID: fe.Name},
		SearchResults: []frontend.SearchResult{{
			Name: "alpha", IP: "192.0.2.1", Servers: []string{fe.UUID}, Versions: []string{"q2pro", "r1q2"},
			Visits: 3, Lookup: "YWxwaGFcMTAw",
		}},
		SearchQuery: "alp",
		PlayerDBInfo: PlayerDatabaseInfo{
			Name: "alpha", IP: "192.0.2.1", ServerUUID: fe.UUID, Userinfo: `\\name\\alpha\\skin\\male/grunt`,
		},
	}
	data.Head.Title = "Test"
	data.RuleHome = serverHome(fe)
	avg := 1.5
	data.Stats = &Stats{
		Span: "24h", Times: []int64{100, 200}, Players: []*float64{nil, &avg}, Kills: []int{0, 3},
		Versions: []Slice{{"q2pro", 2}, {"r1q2", 1}}, Weapons: []Slice{{"railgun", 3}},
		Recent:        []Session{{Name: "alpha", Server: "pf-tdm", Start: 100, Duration: 3700, Frags: 3, Deaths: 1, KDR: 3, Online: true}},
		UniquePlayers: 2, PeakPlayers: 2, TotalKills: 3, AvgSession: 600,
	}
	return data
}

func TestPagesRender(t *testing.T) {
	pages := map[string]string{ // file -> template name
		"dashboard.tmpl":      "dashboard",
		"servers.tmpl":        "my-servers",
		"server-add.tmpl":     "server-add",
		"server-view.tmpl":    "server-view",
		"player-view.tmpl":    "player-view",
		"terminal.tmpl":       "terminal",
		"search.tmpl":         "search",
		"player.tmpl":         "player",
		"rule-list.tmpl":      "rule-list",
		"rule-view.tmpl":      "rule",
		"rule-edit.tmpl":      "rule-edit",
		"generate-keys.tmpl":  "keygen",
		"terms-of-use.tmpl":   "terms-of-use",
		"privacy-policy.tmpl": "privacy-policy",
		"profile.tmpl":        "profile",
	}
	for file, name := range pages {
		t.Run(name, func(t *testing.T) {
			out := renderPage(t, file, name, samplePage())
			if !strings.Contains(out, `data-bs-theme="dark"`) || !strings.Contains(out, "</html>") {
				t.Error("page is missing the shared layout")
			}
		})
	}
}

// signedInClient returns an HTTP client for srv whose requests carry a valid
// session cookie for user.
func signedInClient(t *testing.T, srv *httptest.Server, user *pb.User) *http.Client {
	t.Helper()
	savedUsers := be.users
	be.users = append(be.users, user) // sessions resolve to known users
	t.Cleanup(func() { be.users = savedUsers })
	Website.Secret = []byte("test-secret")
	token, err := CreateSessionToken(user, "test-session", 3600, Website.Secret)
	if err != nil {
		t.Fatal(err)
	}
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(srv.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: CookieName, Value: token}})
	return &http.Client{Jar: jar}
}

// connectedFrontend is a trusted frontend owned by owner whose outgoing
// messages are drained so SendMessages doesn't block.
func connectedFrontend(t *testing.T, owner string) *frontend.Frontend {
	ours, theirs := net.Pipe()
	t.Cleanup(func() { ours.Close(); theirs.Close() })
	go io.Copy(io.Discard, theirs)
	return &frontend.Frontend{
		Name: "fe1", UUID: "fe1-uuid", Owner: owner,
		Connected: true, Trusted: true, Connection: &ours,
		Log: log.New(io.Discard, "", 0), Server: &be,
	}
}

// ruleDB is a temporary database with a rule table, for handlers that save
// rules.
func ruleDB(t *testing.T) *database.Database {
	t.Helper()
	d, err := database.Open(filepath.Join(t.TempDir(), "q2a.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Handle.Close() })
	createRuleTable(t, &d)
	return &d
}

// createRuleTable adds the rule table to d.
func createRuleTable(t *testing.T, d *database.Database) {
	t.Helper()
	_, err := d.Handle.Exec(`CREATE TABLE "rule" (
		"id" INTEGER, "frontend" INTEGER, "uuid" TEXT NOT NULL, "rule_type" INTEGER NOT NULL,
		"creation_time" INTEGER, "expiration_time" INTEGER DEFAULT 0, "delay" INTEGER DEFAULT 0,
		"stifle_length" INTEGER DEFAULT 60, "disabled" INTEGER DEFAULT 1, "scope" TEXT DEFAULT 'local',
		"timespec_before" TEXT, "timespec_after" TEXT, "timespec_every" TEXT, "timespec_playtime" TEXT,
		"addresses" TEXT, "names" TEXT, "hostnames" TEXT, "clients" TEXT, "userinfo" TEXT, "message" TEXT,
		"description" TEXT, "vpn" INTEGER DEFAULT 0, "server_group" INTEGER DEFAULT 0, PRIMARY KEY("id" AUTOINCREMENT))`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestStaticFilesRevalidate(t *testing.T) {
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	res, err := http.Get(srv.URL + "/static/css/q2a.css")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("status %d, Cache-Control %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}
}

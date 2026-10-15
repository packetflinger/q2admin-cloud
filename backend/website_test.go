package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"github.com/packetflinger/q2admind/database"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"

	"github.com/golang-jwt/jwt"
	pb "github.com/packetflinger/q2admind/proto"
)

/*
func TestCreateSessionToken(t *testing.T) {
	tests := []struct {
		name    string
		user    *pb.User
		secret  []byte
		want    string
		wantErr bool
	}{
		{
			name:    "nil user",
			user:    nil,
			secret:  []byte("han shot first"),
			want:    "",
			wantErr: true,
		},
		{
			name:    "nil secret",
			user:    &pb.User{Email: "claire@q2.wtf"},
			secret:  nil,
			want:    "",
			wantErr: true,
		},
		{
			name:    "valid",
			user:    &pb.User{Email: "claire@q2.wtf"},
			secret:  []byte("96b30e93fa4ea5f6a915e48da1da0d3c"),
			want:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjg2NDAxNzYwNDA1OTE4LCJqdGkiOiJ0aGlzIGlzIGFuIGlkIiwic3ViIjoiY2xhaXJlQHEyLnd0ZiJ9.d8xkoUf6RVhQ35YrVRTbUdyUc2kKJPhQC9OTZARnlyg",
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CreateSessionToken(tc.user, tc.secret)
			if tc.wantErr != (err != nil) {
				t.Errorf("CreateSessionToken(%v, %v) resulted in an error: %v", tc.user, tc.secret, err)
			}
			if got != tc.want {
				t.Errorf("CreateSessionToken(%v, %v) = %q, want %q", tc.user, tc.secret, got, tc.want)
			}
		})
	}
}
*/

func TestValidateSessionToken(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		user       *pb.User
		secret     []byte
		wantValid  bool
		wantClaims *jwt.StandardClaims
		wantErr    bool
	}{
		{},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

		})
	}
}

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
	be.config = &pb.Config{WebRoot: "../api/website"}
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
			Name: "alpha", IP: "192.0.2.1", ServerUUID: fe.UUID, Lookup: "YWxwaGFcMTAw",
		}},
		SearchQuery: "alp",
		PlayerDBInfo: PlayerDatabaseInfo{
			Name: "alpha", IP: "192.0.2.1", ServerUUID: fe.UUID, Userinfo: `\\name\\alpha\\skin\\male/grunt`,
		},
	}
	data.Head.Title = "Test"
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

func TestSummarize(t *testing.T) {
	data := samplePage()
	offline := &frontend.Frontend{Name: "down", Players: []frontend.Player{{ConnectTime: 5}}}
	got := summarize(append(data.Frontends, offline))
	want := FrontendSummary{Servers: 2, Online: 1, Players: 1, Rules: 1}
	if got != want {
		t.Errorf("summarize() = %+v, want %+v", got, want)
	}
}

func TestDashboardEmptyState(t *testing.T) {
	data := samplePage()
	data.Frontends = nil
	out := renderPage(t, "dashboard.tmpl", "dashboard", data)
	if !strings.Contains(out, "Add your first server") {
		t.Error("empty dashboard should prompt to add a server")
	}
}

func TestValidateNewServer(t *testing.T) {
	tests := []struct {
		name, addr, port string
		wantName         string
		wantPort         int
		wantErr          bool
	}{
		{"My Server", "100.64.1.2", "", "my-server", 27910, false},
		{"pf-1", "q2.example.com", "27911", "pf-1", 27911, false},
		{"v6", "2001:db8::1", "", "v6", 27910, false},
		{"../../etc", "1.2.3.4", "", "", 0, true},
		{"", "1.2.3.4", "", "", 0, true},
		{"ok", "", "", "", 0, true},
		{"ok", "host:27910", "", "", 0, true},
		{"ok", "1.2.3.4", "70000", "", 0, true},
		{"ok", "1.2.3.4", "abc", "", 0, true},
	}
	for _, tc := range tests {
		name, port, err := validateNewServer(tc.name, tc.addr, tc.port)
		if (err != nil) != tc.wantErr || name != tc.wantName || port != tc.wantPort {
			t.Errorf("validateNewServer(%q, %q, %q) = %q, %d, %v", tc.name, tc.addr, tc.port, name, port, err)
		}
	}
}

func TestAddServerFormShowsError(t *testing.T) {
	data := samplePage()
	data.FormError = "port must be a number from 1 to 65535"
	data.Form = url.Values{"NewServerName": {"keep-me"}}
	out := renderPage(t, "server-add.tmpl", "server-add", data)
	if !strings.Contains(out, data.FormError) || !strings.Contains(out, `value="keep-me"`) {
		t.Error("form error or submitted value missing from re-rendered form")
	}
}

func TestMapNameRE(t *testing.T) {
	for _, ok := range []string{"q2dm1", "tltf", "dm/q2rdm2", "my_map-2"} {
		if !mapNameRE.MatchString(ok) {
			t.Errorf("%q should be allowed", ok)
		}
	}
	// these would inject extra console commands or are empty
	for _, bad := range []string{"", "q2dm1;quit", "q2dm1\nquit", "q2dm1 quit", `"q2dm1"`} {
		if mapNameRE.MatchString(bad) {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestServerViewStates(t *testing.T) {
	data := samplePage()
	out := renderPage(t, "server-view.tmpl", "server-view", data)
	for _, want := range []string{"Change map", ">alpha</a>", "192.0.2.1", "test ban"} {
		if !strings.Contains(out, want) {
			t.Errorf("online server page is missing %q", want)
		}
	}

	data.Frontend.Connected = false
	data.FormError = "port must be a number from 1 to 65535"
	data.Notice = notices["saved"]
	out = renderPage(t, "server-view.tmpl", "server-view", data)
	for _, want := range []string{"isn't connected to CloudAdmin", data.FormError, notices["saved"].Text} {
		if !strings.Contains(out, want) {
			t.Errorf("offline server page is missing %q", want)
		}
	}
	if strings.Contains(out, "Change map") {
		t.Error("offline server page shouldn't offer a map change")
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

func TestConsoleStreamAndCommand(t *testing.T) {
	user := &pb.User{Email: "admin@example.com"}
	fe := connectedFrontend(t, user.Email)
	saved := be.frontends
	be.frontends = []*frontend.Frontend{fe}
	defer func() { be.frontends = saved }()
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, user)
	base := srv.URL + "/sv/fe1-uuid/fe1/console"

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/stream", nil)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(res.Body)
	lines.Scan() // ": connected", so the subscription exists

	post := func(cmd string) int {
		r, err := client.PostForm(base+"/command", url.Values{"cmd": {cmd}})
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if code := post("status"); code != http.StatusNoContent {
		t.Fatalf("command status = %d, want 204", code)
	}
	if code := post("status\nquit"); code != http.StatusBadRequest {
		t.Errorf("multi-line command status = %d, want 400", code)
	}
	for lines.Scan() {
		if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
			var msg string
			if err := json.Unmarshal([]byte(data), &msg); err != nil || !strings.Contains(msg, "> status") {
				t.Errorf("stream got %q (%v), want the echoed command", data, err)
			}
			break
		}
	}
	cancel()
	res.Body.Close()
}

func TestConsoleRejectsOtherOwners(t *testing.T) {
	fe := connectedFrontend(t, "owner@example.com")
	saved := be.frontends
	be.frontends = []*frontend.Frontend{fe}
	defer func() { be.frontends = saved }()
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, &pb.User{Email: "someone@example.com"})
	r, err := client.PostForm(srv.URL+"/sv/fe1-uuid/fe1/console/command", url.Values{"cmd": {"quit"}})
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound { // same as a server that doesn't exist
		t.Errorf("status = %d, want 404", r.StatusCode)
	}
}

func TestPlayerActions(t *testing.T) {
	user := &pb.User{Email: "admin@example.com"}
	fe := connectedFrontend(t, user.Email)
	fe.Players = []frontend.Player{{ClientID: 0, ConnectTime: 1, Name: "alpha"}}
	saved := be.frontends
	be.frontends = []*frontend.Frontend{fe}
	defer func() { be.frontends = saved }()
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, user)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	tests := []struct {
		player     string
		form       url.Values
		wantNotice string
	}{
		{"0", url.Values{"action": {"message"}, "msg": {"hi\nthere"}}, "sent"},
		{"0", url.Values{"action": {"message"}, "msg": {"  "}}, "badinput"},
		{"0", url.Values{"action": {"mute"}, "seconds": {"60"}}, "muted"},
		{"0", url.Values{"action": {"mute"}, "seconds": {"-1"}}, "badinput"},
		{"0", url.Values{"action": {"stifle"}, "seconds": {"301"}}, "badinput"},
		{"0", url.Values{"action": {"stifle"}, "seconds": {"30"}}, "stifled"},
		{"5", url.Values{"action": {"kick"}}, "noplayer"},
		{"0", url.Values{"action": {"kick"}}, "kicked"},
	}
	for _, tc := range tests {
		res, err := client.PostForm(srv.URL+"/sv/fe1-uuid/fe1/player/"+tc.player+"/action", tc.form)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if loc := res.Header.Get("Location"); !strings.Contains(loc, "notice="+tc.wantNotice) {
			t.Errorf("%v on player %s redirected to %q, want notice=%s", tc.form, tc.player, loc, tc.wantNotice)
		}
	}
}

func TestSearchPages(t *testing.T) {
	data := samplePage()
	out := renderPage(t, "search.tmpl", "search", data)
	if !strings.Contains(out, "/player/YWxwaGFcMTAw") || strings.Contains(out, "<th class=\"d-none d-sm-table-cell\">Server</th>") {
		t.Error("server search should list the result without a Server column")
	}
	data.Frontend = nil
	out = renderPage(t, "search.tmpl", "search", data)
	if !strings.Contains(out, "Search all servers") || !strings.Contains(out, ">pf-tdm</td>") {
		t.Error("global search should show which server each result is from")
	}
	out = renderPage(t, "player.tmpl", "player", data)
	if !strings.Contains(out, "male/grunt") {
		t.Error("player record should show parsed userinfo")
	}
}

func TestKeysPage(t *testing.T) {
	data := samplePage()
	data.KeyPair = &KeyPair{Private: "-----BEGIN PRIVATE KEY-----", Public: "-----BEGIN PUBLIC KEY-----"}
	out := renderPage(t, "generate-keys.tmpl", "keygen", data)
	if !strings.Contains(out, "Copy the private key now") || !strings.Contains(out, "BEGIN PRIVATE KEY") {
		t.Error("generated key pair isn't shown")
	}
	data.KeyPair = nil
	if out := renderPage(t, "generate-keys.tmpl", "keygen", data); strings.Contains(out, "Copy the private key now") {
		t.Error("key warning shown with no key pair")
	}
}

func TestInfoPagesRenderSignedOut(t *testing.T) {
	for file, name := range map[string]string{"privacy-policy.tmpl": "privacy-policy", "terms-of-use.tmpl": "terms-of-use"} {
		if out := renderPage(t, file, name, PageResponse{}); strings.Contains(out, "Account menu") {
			t.Errorf("%s shows an account menu with nobody signed in", name)
		}
	}
}

func TestSignInPage(t *testing.T) {
	tmpl := template.Must(template.ParseFiles("../api/website/templates/new/sign-in.tmpl"))
	var out bytes.Buffer
	data := PageResponse{AuthProviders: []AuthProvider{{URL: "/auth/google", Icon: "/g.svg", Name: providerName("GOOGLE"), Enabled: true}, {Name: "Off"}}}
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `you agree to the <a href="/terms-of-use">Terms</a> and <a href="/privacy-policy">Privacy policy</a>`) {
		t.Error("sign-in page is missing the terms and privacy agreement line")
	}
	if !strings.Contains(out.String(), "Continue with Google") || strings.Contains(out.String(), "Off") {
		t.Error("sign-in page should list only enabled providers")
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
		"description" TEXT, "vpn" INTEGER DEFAULT 0, PRIMARY KEY("id" AUTOINCREMENT))`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRulePages(t *testing.T) {
	savedConfig := be.config
	be.config = &pb.Config{WebRoot: "../api/website"}
	defer func() { be.config = savedConfig }()
	user := &pb.User{Email: "admin@example.com"}
	fe := connectedFrontend(t, user.Email)
	fe.Data = ruleDB(t)
	fe.ID = 3
	fe.Players = []frontend.Player{{ClientID: 0, ConnectTime: 1, Name: "alpha", IP: "192.0.2.7"}}
	otherFE := &frontend.Frontend{Name: "theirs", UUID: "theirs-uuid", Owner: "other@example.com",
		Rules: []*pb.Rule{{Uuid: "their-rule", Type: pb.RuleType_BAN, Address: []string{"10.0.0.0/8"}}}}
	saved := be.frontends
	be.frontends = []*frontend.Frontend{fe, otherFE}
	defer func() { be.frontends = saved }()
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, user)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	post := func(path string, form url.Values) (*http.Response, string) {
		t.Helper()
		res, err := client.PostForm(srv.URL+path, form)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res, string(body)
	}
	get := func(path string) int {
		t.Helper()
		res, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	// rejected: nothing to match, then a bad network
	if res, body := post("/sv/fe1-uuid/fe1/rules/add", url.Values{"type": {"ban"}}); res.StatusCode != 200 || !strings.Contains(body, "add at least one thing to match") {
		t.Errorf("rule with no criteria: status %d\n%s", res.StatusCode, body)
	}
	if _, body := post("/sv/fe1-uuid/fe1/rules/add", url.Values{"type": {"ban"}, "addresses": {"10.0.0.0/33"}}); !strings.Contains(body, "isn&#39;t an IP address or CIDR network") {
		t.Error("bad CIDR not rejected")
	}

	res, _ := post("/sv/fe1-uuid/fe1/rules/add", url.Values{
		"type": {"ban"}, "description": {"test"}, "addresses": {"192.0.2.9\n10.0.0.0/8"}, "expires": {"never"},
	})
	if res.StatusCode != http.StatusSeeOther || len(fe.Rules) != 1 {
		t.Fatalf("add: status %d, %d rules", res.StatusCode, len(fe.Rules))
	}
	rule := fe.Rules[0]
	if !slices.Equal(rule.GetAddress(), []string{"192.0.2.9/32", "10.0.0.0/8"}) {
		t.Errorf("addresses saved as %v", rule.GetAddress())
	}
	if code := get("/rules/" + rule.GetUuid() + "/view"); code != 200 {
		t.Errorf("view status %d", code)
	}
	if code := get("/rules/their-rule/view"); code != http.StatusNotFound {
		t.Errorf("someone else's rule: status %d, want 404", code)
	}

	post("/rules/"+rule.GetUuid()+"/edit", url.Values{"type": {"mute"}, "names": {"^bad"}, "expires": {"never"}})
	if fe.Rules[0].GetType() != pb.RuleType_MUTE || len(fe.Rules[0].GetAddress()) != 0 {
		t.Errorf("edit not applied: %v", fe.Rules[0])
	}
	if stored, _ := fe.FetchRules(); len(stored) != 1 || stored[0].GetType() != pb.RuleType_MUTE {
		t.Errorf("edit not saved to the database: %v", stored)
	}

	post("/rules/"+rule.GetUuid()+"/delete", nil)
	if stored, _ := fe.FetchRules(); len(fe.Rules) != 0 || len(stored) != 0 {
		t.Error("delete left the rule behind")
	}

	// the player page's ban adds an IP rule; blackhole only sends a command
	res, _ = post("/sv/fe1-uuid/fe1/player/0/action", url.Values{"action": {"ban"}})
	if !strings.Contains(res.Header.Get("Location"), "notice=banned") || len(fe.Rules) != 1 || fe.Rules[0].GetAddress()[0] != "192.0.2.7/32" {
		t.Errorf("ban: %s, rules %v", res.Header.Get("Location"), fe.Rules)
	}
	res, _ = post("/sv/fe1-uuid/fe1/player/0/action", url.Values{"action": {"blackhole"}})
	if !strings.Contains(res.Header.Get("Location"), "notice=blackholed") {
		t.Errorf("blackhole redirected to %s", res.Header.Get("Location"))
	}
}

func TestStaticFilesRevalidate(t *testing.T) {
	if err := os.Chdir(".."); err != nil { // static files are served relative to the repo root
		t.Fatal(err)
	}
	defer os.Chdir("backend")
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

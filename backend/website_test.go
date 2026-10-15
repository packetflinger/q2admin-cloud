package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
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
	}
	data.Head.Title = "Test"
	return data
}

func TestPagesRender(t *testing.T) {
	pages := map[string]string{ // file -> template name
		"dashboard.tmpl":     "dashboard",
		"servers.tmpl":       "my-servers",
		"server-add.tmpl":    "server-add",
		"server-view.tmpl":   "server-view",
		"player-view.tmpl":   "player-view",
		"terminal.tmpl":      "terminal",
		"search.tmpl":        "search",
		"player.tmpl":        "player",
		"rule-list.tmpl":     "rule-list",
		"rule-view.tmpl":     "rule",
		"rule-add.tmpl":      "rule-add",
		"rule-edit.tmpl":     "rule-edit",
		"generate-keys.tmpl": "keygen",
		"terms-of-use.tmpl":  "terms-of-use",
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
	if r.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", r.StatusCode)
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

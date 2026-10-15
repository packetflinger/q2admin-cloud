package backend

import (
	"bytes"
	"html/template"
	"net"
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

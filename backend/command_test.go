package backend

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

func TestLowerCommandName(t *testing.T) {
	for in, want := range map[string]string{
		"Say Hi There":     "say Hi There",
		"STATUS":           "status",
		"Sv_MaxClients 16": "sv_maxclients 16",
		"say  Two  Spaces": "say  Two  Spaces",
		"":                 "",
	} {
		if got := LowerCommandName(in); got != want {
			t.Errorf("LowerCommandName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConsoleCommandLowercasesName(t *testing.T) {
	user := &pb.User{Email: "admin@example.com"}
	fe := connectedFrontend(t, user.Email)
	ours, theirs := net.Pipe() // capture what's sent to the frontend
	t.Cleanup(func() { ours.Close(); theirs.Close() })
	fe.Connection = &ours
	sent := make(chan []byte, 1)
	go func() {
		frame, _ := ReadFrame(theirs)
		sent <- frame
		io.Copy(io.Discard, theirs)
	}()
	saved := be.frontends
	be.frontends = []*frontend.Frontend{fe}
	defer func() { be.frontends = saved }()
	watch := make(chan frontend.ConsoleLine, 4)
	fe.AddTerminal(&watch, 0)
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, user)

	res, err := client.PostForm(srv.URL+"/sv/fe1-uuid/fe1/console/command", url.Values{"cmd": {"Say Hi There"}})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d", res.StatusCode)
	}
	if frame := <-sent; !bytes.Contains(frame, []byte("say Hi There\n")) {
		t.Errorf("frontend was sent %q, want the command lowercased and the text untouched", frame)
	}
	if echo := (<-watch).Text; !strings.HasSuffix(echo, "> say Hi There") {
		t.Errorf("console watchers saw %q first, want the command", echo)
	}
	if echo := (<-watch).Text; !strings.Contains(echo, "console: Hi There") {
		t.Errorf("console watchers saw %q second, want the say echo", echo)
	}
}

func TestConsoleCommandEchoesSay(t *testing.T) {
	fe := &frontend.Frontend{}
	watch := make(chan frontend.ConsoleLine, 4)
	fe.AddTerminal(&watch, 0)
	for _, tc := range []struct{ cmd, echo string }{
		{"say hi there", "console: hi there"},
		{`Say "quoted text"`, "console: quoted text"},
		{"say", ""},           // nothing to say
		{"gamemap q2dm1", ""}, // only say echoes
		{"sayteam hi", ""},
	} {
		ConsoleCommand(fe, tc.cmd)
		got := ""
		select {
		case line := <-watch:
			got = line.Text
		default:
		}
		if (tc.echo == "") != (got == "") || !strings.Contains(got, tc.echo) {
			t.Errorf("ConsoleCommand(%q) echoed %q, want %q", tc.cmd, got, tc.echo)
		}
	}
	if sent := string(fe.MessageOut.Data); !strings.Contains(sent, "say \"quoted text\"\n") {
		t.Errorf("the command sent should keep its quotes, got %q", sent)
	}
}

func TestConsoleStatus(t *testing.T) {
	conn, _ := net.Pipe()
	defer conn.Close()
	fe := &frontend.Frontend{IPAddress: "100.64.1.2", Port: 27910, Connection: &conn, CurrentMap: "q2dm1"}
	fe.Players = []frontend.Player{
		{ClientID: 0, ConnectTime: 1, Name: "alpha", IP: "192.0.2.1", Frags: 12},
		{ClientID: 1}, // empty slot
		{ClientID: 2, ConnectTime: 1, Name: "bravo", IP: "192.0.2.2", Frags: 3},
	}
	watch := make(chan frontend.ConsoleLine, 4)
	fe.AddTerminal(&watch, 0)
	status := func() string {
		t.Helper()
		ConsoleCommand(fe, "Status")
		select {
		case line := <-watch:
			return line.Text
		default:
			t.Fatal("status printed nothing")
			return ""
		}
	}

	out := status()
	for _, want := range []string{"q2dm1", "  0    12 alpha", "192.0.2.1\n", "  2     3 bravo", "192.0.2.2"} {
		if !strings.Contains(out, want) {
			t.Errorf("status is missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "\n  ") < 2 { // each player on their own line
		t.Errorf("players should be on separate lines:\n%s", out)
	}
	if len(fe.MessageOut.Data) != 0 {
		t.Errorf("status shouldn't be sent to the server, sent %q", fe.MessageOut.Data)
	}

	fe.Players = nil
	fe.Connection = nil
	if out := status(); !strings.Contains(out, "No UDP clients") || !strings.Contains(out, "not connected") {
		t.Errorf("empty, disconnected status:\n%s", out)
	}
}

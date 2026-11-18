package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

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
	if res.Header.Get("X-Accel-Buffering") != "no" {
		t.Error("stream should tell nginx not to buffer it")
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
	event := ""
	for lines.Scan() {
		if name, ok := strings.CutPrefix(lines.Text(), "event: "); ok {
			event = name
			continue
		}
		if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
			if event != "" { // a status update, not console output
				event = ""
				continue
			}
			var msg struct{ M string }
			if err := json.Unmarshal([]byte(data), &msg); err != nil || !strings.Contains(msg.M, "> status") {
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

func TestConsoleStreamReplaysBacklog(t *testing.T) {
	user := &pb.User{Email: "admin@example.com"}
	fe := connectedFrontend(t, user.Email)
	saved := be.frontends
	be.frontends = []*frontend.Frontend{fe}
	defer func() { be.frontends = saved }()
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, user)
	for _, s := range []string{"one", "two", "three"} {
		fe.ConsoleBroadcast(s)
	}

	// open reads a stream, sending lastID if it isn't empty, until it has n
	// lines, and returns their ids and texts
	open := func(lastID string, n int, live string) (ids, texts []string) {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/sv/fe1-uuid/fe1/console/stream", nil)
		if lastID != "" {
			req.Header.Set("Last-Event-ID", lastID)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		lines := bufio.NewScanner(res.Body)
		lines.Scan() // ": connected", so the subscription exists
		if live != "" {
			fe.ConsoleBroadcast(live)
		}
		for len(texts) < n && lines.Scan() {
			if lines.Text() == "event: status" {
				lines.Scan() // its data line, not console output
				continue
			}
			if id, ok := strings.CutPrefix(lines.Text(), "id: "); ok {
				ids = append(ids, id)
			}
			if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
				var line struct {
					T int64
					M string
				}
				if err := json.Unmarshal([]byte(data), &line); err != nil || line.T == 0 {
					t.Fatalf("bad line %q: %v", data, err)
				}
				texts = append(texts, line.M)
			}
		}
		return ids, texts
	}

	ids, texts := open("", 4, "four")
	if strings.Join(texts, ",") != "one,two,three,four" || len(ids) != 4 {
		t.Fatalf("a new console got ids %v, lines %v; want the backlog then the live line", ids, texts)
	}
	if _, texts := open(ids[1], 2, ""); strings.Join(texts, ",") != "three,four" {
		t.Errorf("resuming after id %s gave %v, want only what came after", ids[1], texts)
	}
}

// The stream sends the console header's map and player count when it opens
// and again when they change.
func TestConsoleStreamStatus(t *testing.T) {
	saved := consoleStatusEvery
	consoleStatusEvery = 10 * time.Millisecond
	defer func() { consoleStatusEvery = saved }()
	user := &pb.User{Email: "admin@example.com"}
	fe := connectedFrontend(t, user.Email)
	fe.CurrentMap, fe.MaxPlayers = "q2dm1", 8
	fe.Players = []frontend.Player{{ClientID: 0, ConnectTime: 1, Name: "alpha"}}
	savedFEs := be.frontends
	be.frontends = []*frontend.Frontend{fe}
	defer func() { be.frontends = savedFEs }()
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, user)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/sv/fe1-uuid/fe1/console/stream", nil)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	lines := bufio.NewScanner(res.Body)
	next := func() consoleStatus {
		t.Helper()
		for lines.Scan() {
			if lines.Text() != "event: status" {
				continue
			}
			lines.Scan()
			var st consoleStatus
			if err := json.Unmarshal([]byte(strings.TrimPrefix(lines.Text(), "data: ")), &st); err != nil {
				t.Fatal(err)
			}
			return st
		}
		t.Fatal("stream ended without a status event")
		return consoleStatus{}
	}
	if st := next(); st != (consoleStatus{Online: true, Map: "q2dm1", Players: 1, Max: 8}) {
		t.Errorf("first status = %+v", st)
	}
	fe.Mu.Lock()
	fe.CurrentMap = "q2dm2"
	fe.Mu.Unlock()
	if st := next(); st.Map != "q2dm2" || st.Players != 1 {
		t.Errorf("after the map change, status = %+v", st)
	}
	fe.Mu.Lock()
	fe.Connected = false
	fe.Mu.Unlock()
	if st := next(); st.Online {
		t.Errorf("after the server dropped, status = %+v", st)
	}
}

func TestConsolePageShowsMapAndPlayers(t *testing.T) {
	out := renderPage(t, "terminal.tmpl", "terminal", samplePage())
	for _, want := range []string{`<span id="console-map">q2dm1</span>`, `<span id="console-players">1/2</span>`} {
		if !strings.Contains(out, want) {
			t.Errorf("console page is missing %q", want)
		}
	}
}

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
	for lines.Scan() {
		if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
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

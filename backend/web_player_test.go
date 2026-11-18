package backend

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

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

func TestPlayerViewLinksIP(t *testing.T) {
	data := samplePage()
	data.Player.UserinfoMap = map[string]string{"ip": "192.0.2.1:27901", "name": "alpha"}
	out := renderPage(t, "player-view.tmpl", "player-view", data)
	link := `<a href="https://lens.ipverse.net/192.0.2.1" target="_blank" rel="noopener noreferrer">192.0.2.1</a>`
	if n := strings.Count(out, link); n != 2 {
		t.Errorf("found %d IP links, want 2 (connection and userinfo)", n)
	}
	if !strings.Contains(out, link+":27901") {
		t.Error("userinfo ip lost its port")
	}
}

func TestPlayerViewUnknownAddress(t *testing.T) {
	data := samplePage()
	data.Player.IP, data.Player.Port = "", 0
	out := renderPage(t, "player-view.tmpl", "player-view", data)
	if !strings.Contains(out, `<dt>Address</dt><dd class="q2a-mono">-</dd>`) {
		t.Error("an unknown address should show as -")
	}
}

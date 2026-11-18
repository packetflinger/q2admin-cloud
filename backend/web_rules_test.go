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

func TestRulePages(t *testing.T) {
	savedConfig := be.config
	be.config = &pb.Config{}
	defer func() { be.config = savedConfig }()
	user := &pb.User{Email: "admin@example.com"}
	fe := connectedFrontend(t, user.Email)
	fe.Data = ruleDB(t)
	fe.ID = 3
	savedDB := db
	db = *fe.Data // rule lookups also search the user's groups
	defer func() { db = savedDB }()
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

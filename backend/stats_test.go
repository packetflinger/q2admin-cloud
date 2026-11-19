package backend

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

// statsTestSetup is a fresh database with the stats tables, the frontend
// and player_stat tables that the server and the game write, and five
// visits: two by alpha on different servers, one by bravo, one on someone
// else's server, and one too old for the 24 hour span. fe1 has a player
// connected.
func statsTestSetup(t *testing.T) (up, down, theirs *frontend.Frontend) {
	t.Helper()
	up, down, theirs = groupTestSetup(t, "admin@example.com")
	up.Name, down.Name = "fe1", "fe2"
	up.Players = []frontend.Player{{ClientID: 0, ConnectTime: time.Now().Unix() - 120, Name: "live", Frags: 4, Deaths: 2}}
	for _, q := range []string{
		`CREATE TABLE frontend (id INTEGER, name TEXT)`,
		`INSERT INTO frontend VALUES (1, 'fe1'), (2, 'fe2'), (3, 'theirs')`,
		`CREATE TABLE player_stat (id INTEGER PRIMARY KEY, player INTEGER, frags INTEGER, deaths INTEGER, suicides INTEGER, kdr INTEGER, play_time INTEGER)`,
	} {
		if _, err := db.Handle.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Unix()
	// alpha's visits use one client, bravo's another
	visits := []struct {
		server, name, ip, version string
		ago, played               int64
	}{
		{"fe1", "alpha", "192.0.2.1", "q2pro", 3600, 600},
		{"fe2", "alpha", "192.0.2.1", "q2pro", 1800, 1200},
		{"fe1", "bravo", "192.0.2.2", "r1q2", 600, 300},
		{"theirs", "zulu", "192.0.2.9", "egl", 600, 300},
		{"fe1", "old", "192.0.2.3", "q2pro", 3 * 86400, 300},
	}
	for i, v := range visits {
		db.Handle.Exec(`INSERT INTO player (id, server, name, ip, version, time) VALUES (?, ?, ?, ?, ?, ?)`, i+1, v.server, v.name, v.ip, v.version, now-v.ago)
		db.Handle.Exec(`INSERT INTO player_stat (player, frags, deaths, play_time) VALUES (?, 2, 1, ?)`, i+1, v.played)
	}
	return up, down, theirs
}

func TestStats(t *testing.T) {
	up, down, theirs := statsTestSetup(t)
	now := time.Now().Unix()
	samples := []struct {
		ago  int64
		fe   *frontend.Frontend
		n    int
		kill map[string]int
	}{
		{40 * 86400, up, 50, nil}, // pruned by the later saves
		{7200, up, 3, map[string]int{"railgun": 2}},
		{7200, down, 1, map[string]int{"rocket launcher": 1}},
		{60, up, 2, map[string]int{"railgun": 1}},
		{60, theirs, 9, map[string]int{"bfg": 9}},
	}
	for _, s := range samples {
		if err := saveSamples(now-s.ago, []statsSample{{s.fe.ID, s.n, s.kill}}); err != nil {
			t.Fatal(err)
		}
	}

	st, err := loadStats([]*frontend.Frontend{up, down}, "bogus", false) // falls back to 24h
	if err != nil {
		t.Fatal(err)
	}
	if st.Span != "24h" || len(st.Times) != statBuckets {
		t.Errorf("span %q with %d buckets", st.Span, len(st.Times))
	}
	if st.PeakPlayers != 4 || st.TotalKills != 4 {
		t.Errorf("peak %d, kills %d; want 4, 4 (theirs excluded)", st.PeakPlayers, st.TotalKills)
	}
	if got := fmt.Sprint(st.Weapons); got != "[{railgun 3} {rocket launcher 1}]" {
		t.Errorf("weapons = %s", got)
	}
	if got := fmt.Sprint(st.Versions); got != "[{q2pro 1} {r1q2 1}]" {
		t.Errorf("versions = %s, want each player counted once", got)
	}
	if st.UniquePlayers != 2 || st.AvgSession != 700 {
		t.Errorf("unique %d, avg session %d; want 2, 700", st.UniquePlayers, st.AvgSession)
	}
	var names []string
	for _, s := range st.Recent {
		names = append(names, s.Name)
	}
	if strings.Join(names, ",") != "live,bravo,alpha,alpha" || !st.Recent[0].Online || st.Recent[0].KDR != 2 {
		t.Errorf("recent = %v", st.Recent)
	}
	var n int
	db.Handle.QueryRow("SELECT COUNT(*) FROM server_sample WHERE players = 50").Scan(&n)
	if n != 0 {
		t.Error("samples older than 31 days weren't pruned")
	}
}

// The dashboard shows each player once, at the latest server they joined,
// with a player still connected counting as their latest.
func TestStatsRecentPerPlayer(t *testing.T) {
	up, down, _ := statsTestSetup(t)
	recent := func() string {
		t.Helper()
		st, err := loadStats([]*frontend.Frontend{up, down}, "24h", true)
		if err != nil {
			t.Fatal(err)
		}
		var rows []string
		for _, s := range st.Recent {
			rows = append(rows, fmt.Sprintf("%s@%s %d", s.Name, s.Server, s.Duration))
		}
		return strings.Join(rows, ",")
	}
	if got := recent(); !strings.HasPrefix(got, "live@fe1 ") || !strings.HasSuffix(got, ",bravo@fe1 300,alpha@fe2 1200") {
		t.Errorf("recent = %s; want alpha once, at fe2", got)
	}
	down.Players = []frontend.Player{{ClientID: 0, ConnectTime: time.Now().Unix() - 60, Name: "bravo"}}
	if got := recent(); !strings.HasPrefix(got, "bravo@fe2 ") || strings.Contains(got, "bravo@fe1") {
		t.Errorf("recent = %s; want bravo once, playing on fe2", got)
	}
}

func TestLoadSessionsForPlayer(t *testing.T) {
	up, down, theirs := statsTestSetup(t)
	fes := []*frontend.Frontend{up, down, theirs}
	servers := func(name string, limit int) string {
		t.Helper()
		ss, err := loadSessions(fes, 0, name, limit, false)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, s := range ss {
			out = append(out, s.Server)
		}
		return strings.Join(out, ",")
	}
	if got := servers("alpha", 20); got != "fe2,fe1" {
		t.Errorf("alpha's servers = %s, want fe2,fe1", got)
	}
	if got := servers("alpha", 1); got != "fe2" {
		t.Errorf("alpha's latest server = %s, want fe2", got)
	}
	if got := servers("live", 20); got != "fe1" {
		t.Errorf("a connected player's servers = %s, want fe1", got)
	}
	if got := servers("old", 20); got != "fe1" {
		t.Errorf("old's servers = %s; the page isn't limited to a span", got)
	}
}

func TestRecentPlayerPage(t *testing.T) {
	statsTestSetup(t)
	db.Handle.Exec(`INSERT INTO player (id, server, name, ip, hostname, time) VALUES (10, 'fe1', 'a/b c', '198.51.100.7', 'host.example.net', 100), (11, 'fe2', 'a/b c', NULL, NULL, 200)`)
	db.Handle.Exec(`INSERT INTO player_stat (player, frags, deaths, play_time) VALUES (10, 3, 1, 60), (11, 1, 1, 60)`)
	db.Handle.Exec(`INSERT INTO player (id, server, name, time) VALUES (12, 'theirs', 'a/b c', 300)`)
	db.Handle.Exec(`INSERT INTO player_stat (player, frags, deaths, play_time) VALUES (12, 0, 0, 60)`)
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()
	client := signedInClient(t, srv, &pb.User{Email: "admin@example.com"})
	get := func(query string) string {
		t.Helper()
		res, err := client.Get(srv.URL + "/recent-players/" + url.PathEscape("a/b c") + query)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status %d", res.StatusCode)
		}
		return string(body)
	}
	out := get("")
	fe2, fe1 := strings.Index(out, "<td>fe2</td>"), strings.Index(out, "<td>fe1</td>")
	if fe2 < 0 || fe1 < fe2 || strings.Contains(out, "<td>theirs</td>") {
		t.Error("page should list fe2 then fe1, and not someone else's server")
	}
	if !strings.Contains(out, `<option value="20" selected>`) {
		t.Error("the limit should default to 20")
	}
	if !strings.Contains(out, `<a href="https://lens.ipverse.net/198.51.100.7" target="_blank" rel="noopener noreferrer">198.51.100.7</a>`) || !strings.Contains(out, "host.example.net") {
		t.Error("page should show each visit's IP, linked, and hostname")
	}
	out = get("?limit=10")
	if !strings.Contains(out, `<option value="10" selected>`) {
		t.Error("limit=10 isn't selected")
	}
	if out = get("?limit=1"); strings.Contains(out, `<option value="1" selected>`) || !strings.Contains(out, `<option value="20" selected>`) {
		t.Error("a limit that isn't a choice should fall back to 20")
	}
}

func TestRecentPlayersLinked(t *testing.T) {
	data := samplePage()
	data.Stats.Recent = []Session{{Name: "a/b c", Server: "pf-tdm"}}
	out := renderPage(t, "dashboard.tmpl", "dashboard", data)
	if !strings.Contains(out, `<a href="/recent-players/a%2Fb%20c"`) {
		t.Error("recent player isn't linked to their page")
	}
}

func TestPieFoldsIntoOther(t *testing.T) {
	counts := map[string]int{}
	for i := range 10 {
		counts[fmt.Sprint("w", i)] = 10 - i
	}
	got := pie(counts)
	if len(got) != pieSlices+1 || got[0] != (Slice{"w0", 10}) || got[pieSlices] != (Slice{"Other", 6}) {
		t.Errorf("pie = %v", got)
	}
	if got := pie(map[string]int{"a": 1, "b": 0}); len(got) != 1 {
		t.Errorf("zero slices kept: %v", got)
	}
}

func TestStatsDataEmbedded(t *testing.T) {
	out := renderPage(t, "dashboard.tmpl", "dashboard", samplePage())
	if !strings.Contains(out, `"Players":[null,1.5]`) || !strings.Contains(out, "1h 01m") {
		t.Error("dashboard is missing the stats data or formatted durations")
	}
}

// The averages are pointers (nil for no samples), so they need formatting
// rather than printing the pointer.
func TestStatsShowDataAverages(t *testing.T) {
	out := renderPage(t, "dashboard.tmpl", "dashboard", samplePage())
	if strings.Contains(out, "%!") {
		t.Error("show data table has a formatting error")
	}
	if !strings.Contains(out, `<td class="text-end">1.5</td>`) || !strings.Contains(out, `<td class="text-end">-</td>`) {
		t.Error("show data table should list 1.5 and - for the two buckets")
	}
}

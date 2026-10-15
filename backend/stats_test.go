package backend

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/packetflinger/q2admind/frontend"
)

func TestStats(t *testing.T) {
	owner := "admin@example.com"
	up, down, theirs := groupTestSetup(t, owner) // fresh database with the stats tables
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
	// two visits by one player with one client, one by another with a different client,
	// one on someone else's server, and one too old to count
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

	st, err := loadStats([]*frontend.Frontend{up, down}, "bogus") // falls back to 24h
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

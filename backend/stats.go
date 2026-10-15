// Stats for the dashboard and server pages: players and kills over time,
// client versions, weapons and recent players, for a chosen time span.
//
// Players and kills are sampled by the maintenance thread (saveSamples);
// versions and sessions come from the player tables written as players
// connect and leave.
package backend

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/packetflinger/q2admind/frontend"
)

// StatSpans are the selectable time spans, shortest first.
var StatSpans = []struct {
	Key   string
	Label string
	Span  time.Duration
}{
	{"1h", "1 hour", time.Hour},
	{"6h", "6 hours", 6 * time.Hour},
	{"24h", "24 hours", 24 * time.Hour},
	{"7d", "7 days", 7 * 24 * time.Hour},
	{"30d", "30 days", 30 * 24 * time.Hour},
}

const (
	defaultSpan    = "24h"
	statBuckets    = 48             // points per line chart, whatever the span
	sampleKeep     = 31 * 24 * 3600 // seconds of samples kept, a bit over the longest span
	pieSlices      = 7              // more than this folds into "Other"
	recentSessions = 25
)

// Stats is everything the stats widgets show for one span.
type Stats struct {
	Span          string
	Times         []int64    // start of each bucket, unix seconds
	Players       []*float64 // average players connected per bucket, nil with no samples
	Kills         []int      // kills per bucket
	Versions      []Slice
	Weapons       []Slice
	Recent        []Session
	UniquePlayers int
	PeakPlayers   int
	TotalKills    int
	AvgSession    int64 // seconds, of sessions that ended
}

// Slice is one wedge of a pie chart.
type Slice struct {
	Label string
	Value int
}

// Session is one player's visit to a server.
type Session struct {
	Name     string
	Server   string
	Start    int64
	Duration int64 // seconds
	Frags    int
	Deaths   int
	KDR      float64
	Online   bool // still playing
}

type statsSample struct {
	frontendID int
	players    int
	kills      map[string]int
}

// saveSamples records one maintenance run's samples and drops samples older
// than any span can show.
// shortcut: kills since the last run are lost if the server stops between
// runs, fine at a few minutes per run.
func saveSamples(now int64, samples []statsSample) error {
	tx, err := db.Handle.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, s := range samples {
		if _, err := tx.Exec("INSERT INTO server_sample (time, frontend_id, players) VALUES (?, ?, ?)", now, s.frontendID, s.players); err != nil {
			return err
		}
		for weapon, kills := range s.kills {
			if _, err := tx.Exec("INSERT INTO frag_sample (time, frontend_id, weapon, kills) VALUES (?, ?, ?, ?)", now, s.frontendID, weapon, kills); err != nil {
				return err
			}
		}
	}
	for _, table := range []string{"server_sample", "frag_sample"} {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE time < ?", now-sampleKeep); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// spanFor returns the span for a ?span= key, falling back to 24 hours.
func spanFor(key string) (string, time.Duration) {
	for _, s := range StatSpans {
		if s.Key == key {
			return s.Key, s.Span
		}
	}
	return spanFor(defaultSpan)
}

// loadStats gathers the stats for fes over the span named by key. It locks
// each frontend briefly, so call it before rendering, not during.
func loadStats(fes []*frontend.Frontend, key string) (*Stats, error) {
	key, span := spanFor(key)
	now := time.Now().Unix()
	start := now - int64(span.Seconds())
	width := int64(span.Seconds()) / statBuckets
	st := &Stats{Span: key, Times: make([]int64, statBuckets), Players: make([]*float64, statBuckets), Kills: make([]int, statBuckets)}
	for i := range st.Times {
		st.Times[i] = start + int64(i)*width
	}
	if len(fes) == 0 {
		return st, nil
	}

	var ids []any
	for _, fe := range fes {
		fe.Mu.Lock()
		ids = append(ids, fe.ID)
		for _, p := range fe.Players {
			if p.ConnectTime > 0 {
				st.Recent = append(st.Recent, Session{Name: p.Name, Server: fe.Name, Start: p.ConnectTime,
					Duration: now - p.ConnectTime, Frags: p.Frags, Deaths: p.Deaths, KDR: frontend.KDR(p.Frags, p.Deaths), Online: true})
			}
		}
		fe.Mu.Unlock()
	}
	in := "(" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")"
	args := append([]any{start}, ids...)
	bucket := func(t int64) int { return min(int((t-start)/width), statBuckets-1) }

	// players: sum across servers per sample, then average per bucket
	rows, err := db.Handle.Query("SELECT time, SUM(players) FROM server_sample WHERE time >= ? AND frontend_id IN "+in+" GROUP BY time", args...)
	if err != nil {
		return nil, fmt.Errorf("reading player samples: %v", err)
	}
	var sum [statBuckets]float64
	var count [statBuckets]int
	for rows.Next() {
		var t int64
		var players int
		if err := rows.Scan(&t, &players); err != nil {
			rows.Close()
			return nil, err
		}
		sum[bucket(t)] += float64(players)
		count[bucket(t)]++
		st.PeakPlayers = max(st.PeakPlayers, players)
	}
	rows.Close()
	for i := range sum {
		if count[i] > 0 {
			avg := sum[i] / float64(count[i])
			st.Players[i] = &avg
		}
	}

	rows, err = db.Handle.Query("SELECT time, weapon, SUM(kills) FROM frag_sample WHERE time >= ? AND frontend_id IN "+in+" GROUP BY time, weapon", args...)
	if err != nil {
		return nil, fmt.Errorf("reading kill samples: %v", err)
	}
	weapons := map[string]int{}
	for rows.Next() {
		var t int64
		var weapon string
		var kills int
		if err := rows.Scan(&t, &weapon, &kills); err != nil {
			rows.Close()
			return nil, err
		}
		st.Kills[bucket(t)] += kills
		weapons[weapon] += kills
		st.TotalKills += kills
	}
	rows.Close()
	st.Weapons = pie(weapons)

	// player.server holds the frontend name; a player is a name+IP pair
	players := "FROM player AS p JOIN frontend AS f ON f.name = p.server WHERE p.time >= ? AND f.id IN " + in
	rows, err = db.Handle.Query("SELECT COALESCE(NULLIF(p.version, ''), 'unknown'), COUNT(DISTINCT p.name || '\\' || p.ip) "+players+" GROUP BY 1", args...)
	if err != nil {
		return nil, fmt.Errorf("reading client versions: %v", err)
	}
	versions := map[string]int{}
	for rows.Next() {
		var v string
		var n int
		if err := rows.Scan(&v, &n); err != nil {
			rows.Close()
			return nil, err
		}
		versions[v] = n
	}
	rows.Close()
	st.Versions = pie(versions)

	err = db.Handle.QueryRow("SELECT COUNT(DISTINCT p.name || '\\' || p.ip) "+players, args...).Scan(&st.UniquePlayers)
	if err != nil {
		return nil, fmt.Errorf("counting players: %v", err)
	}
	ended := "FROM player AS p JOIN frontend AS f ON f.name = p.server JOIN player_stat AS s ON s.player = p.id WHERE p.time >= ? AND f.id IN " + in
	if err := db.Handle.QueryRow("SELECT CAST(COALESCE(AVG(s.play_time), 0) AS INTEGER) "+ended, args...).Scan(&st.AvgSession); err != nil {
		return nil, fmt.Errorf("averaging sessions: %v", err)
	}
	rows, err = db.Handle.Query("SELECT p.name, p.server, p.time, s.play_time, s.frags, s.deaths "+ended+" ORDER BY p.time DESC LIMIT ?", append(args, recentSessions)...)
	if err != nil {
		return nil, fmt.Errorf("reading sessions: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.Name, &s.Server, &s.Start, &s.Duration, &s.Frags, &s.Deaths); err != nil {
			return nil, err
		}
		s.KDR = frontend.KDR(s.Frags, s.Deaths)
		st.Recent = append(st.Recent, s)
	}
	// players still connected first, then the latest to leave
	slices.SortStableFunc(st.Recent, func(a, b Session) int {
		if a.Online != b.Online {
			if a.Online {
				return -1
			}
			return 1
		}
		return cmp.Compare(b.Start, a.Start)
	})
	st.Recent = st.Recent[:min(len(st.Recent), recentSessions)]
	return st, rows.Err()
}

// pie sorts counts largest first, folding everything past pieSlices into
// "Other" so a pie never needs more than 8 colors.
func pie(counts map[string]int) []Slice {
	var out []Slice
	for label, n := range counts {
		if n > 0 {
			out = append(out, Slice{label, n})
		}
	}
	slices.SortFunc(out, func(a, b Slice) int {
		return cmp.Or(cmp.Compare(b.Value, a.Value), cmp.Compare(a.Label, b.Label))
	})
	if len(out) > pieSlices+1 {
		other := Slice{Label: "Other"}
		for _, s := range out[pieSlices:] {
			other.Value += s.Value
		}
		out = append(out[:pieSlices], other)
	}
	return out
}

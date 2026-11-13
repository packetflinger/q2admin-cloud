package backend

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestConnectionHistory(t *testing.T) {
	up, down, _ := groupTestSetup(t, "admin@example.com") // fresh database
	now := time.Now().Unix()
	start := now - 1000

	if h, err := loadConnectionHistory(up, start); err != nil || h != nil {
		t.Fatalf("with no history got %+v, %v; want nil", h, err)
	}

	// connected before the span, dropped twice in it, up again now
	logConnectionAt(start-500, up.ID, true, "")
	logConnectionAt(start+100, up.ID, false, "read error: reset")
	logConnectionAt(start+200, up.ID, true, "")
	logConnectionAt(start+600, up.ID, false, "closed by the server")
	logConnectionAt(start+800, up.ID, true, "")
	logConnectionAt(start+100, down.ID, false, "someone else's")

	h, err := loadConnectionHistory(up, start)
	if err != nil {
		t.Fatal(err)
	}
	var pts []string
	for _, p := range h.Points {
		pts = append(pts, fmt.Sprintf("%d:%d", p.T-start, p.Up))
	}
	if got := strings.Join(pts, " "); !strings.HasPrefix(got, "0:1 100:0 200:1 600:0 800:1 ") {
		t.Errorf("points = %s", got)
	}
	if last := h.Points[len(h.Points)-1]; last.Up != 1 || last.T < now {
		t.Errorf("line should end up, now; ends %+v", last)
	}
	if h.Drops != 2 || len(h.Events) != 4 || h.Events[0].Time != start+800 || h.Events[3].Reason != "read error: reset" {
		t.Errorf("drops %d, events %+v", h.Drops, h.Events)
	}
	// up for 100 + 400 + about 200 of about 1000 seconds
	if h.Uptime < 69 || h.Uptime > 71 {
		t.Errorf("uptime = %.1f%%, want about 70%%", h.Uptime)
	}

	// a history that starts inside the span only counts from its first event
	logConnectionAt(start+500, down.ID, true, "")
	h, _ = loadConnectionHistory(down, start)
	// up for about 500 of the 900 seconds since
	if h.Points[0].T != start+100 || h.Uptime < 55 || h.Uptime > 56 {
		t.Errorf("down: first point %d, uptime %.1f; want start+100 and about 55.6%%", h.Points[0].T-start, h.Uptime)
	}
}

// A connection still marked up when we start was cut off by us stopping. It
// ends at its last stats sample.
func TestCloseOpenConnections(t *testing.T) {
	up, down, theirs := groupTestSetup(t, "admin@example.com")
	logConnectionAt(100, up.ID, true, "")
	saveSamples(400, []statsSample{{frontendID: up.ID}}) // still up then
	logConnectionAt(100, down.ID, true, "")              // no samples
	logConnectionAt(100, theirs.ID, true, "")
	logConnectionAt(150, theirs.ID, false, "closed by the server")

	if err := closeOpenConnections(); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Handle.Query("SELECT frontend_id, time, connected, reason FROM frontend_connection ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var id, up int
		var tm int64
		var reason string
		rows.Scan(&id, &tm, &up, &reason)
		got = append(got, fmt.Sprintf("%d@%d=%d %s", id, tm, up, reason))
	}
	want := "1@100=1 |2@100=1 |3@100=1 |3@150=0 closed by the server|1@400=0 CloudAdmin stopped|2@100=0 CloudAdmin stopped"
	if strings.Join(got, "|") != want {
		t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "|"), want)
	}
}

func TestServerViewConnectionPanel(t *testing.T) {
	data := samplePage()
	out := renderPage(t, "server-view.tmpl", "server-view", data)
	if !strings.Contains(out, "No connection history yet.") || strings.Contains(out, "q2a-connection-chart") {
		t.Error("without history the panel should say so and draw no chart")
	}
	data.Connection = &ConnectionHistory{Start: 0, End: 100, Uptime: 99.25, Drops: 1,
		Points: []ConnPoint{{0, 1}, {50, 0}, {60, 1}, {100, 1}},
		Events: []ConnEvent{{60, true, ""}, {50, false, "read error: reset"}}}
	out = renderPage(t, "server-view.tmpl", "server-view", data)
	for _, want := range []string{"q2a-connection-chart", "99.2%", "<strong>1</strong>", "read error: reset", `"Points":[{"T":0,"Up":1}`} {
		if !strings.Contains(out, want) {
			t.Errorf("connection panel is missing %q", want)
		}
	}
	data.Frontend.Connected = false // shown while offline too
	if out = renderPage(t, "server-view.tmpl", "server-view", data); !strings.Contains(out, "q2a-connection-chart") {
		t.Error("connection chart should show while the server is offline")
	}
}

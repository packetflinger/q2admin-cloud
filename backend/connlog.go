// Connection history: each time a frontend's connection to us becomes
// trusted or ends is saved (see the frontend_connection table), so its page
// can chart when it was up and down.
package backend

import (
	"fmt"
	"time"

	"github.com/packetflinger/q2admind/frontend"
)

const connectionEventsShown = 50 // most recent connects and disconnects listed

// ConnectionHistory is one frontend's connection to us over a span, for the
// up/down chart on its page.
type ConnectionHistory struct {
	Start, End int64       // the span, unix seconds
	Points     []ConnPoint // the line: each change of state, oldest first
	Events     []ConnEvent // connects and disconnects in the span, newest first
	Uptime     float64     // percent of the known part of the span it was connected
	Drops      int         // disconnects in the span
}

// ConnPoint is the connection's state (Up is 1 or 0) from time T onward.
type ConnPoint struct {
	T  int64
	Up int
}

// ConnEvent is one connect or disconnect.
type ConnEvent struct {
	Time      int64
	Connected bool
	Reason    string // why it disconnected
}

// logConnection saves fe's connection becoming trusted (connected) or ending,
// with why it ended.
func logConnection(fe *frontend.Frontend, connected bool, reason string) {
	logConnectionAt(time.Now().Unix(), fe.ID, connected, reason)
}

func logConnectionAt(t int64, frontendID int, connected bool, reason string) {
	if db.Handle == nil {
		return
	}
	up := 0
	if connected {
		up = 1
	}
	_, err := db.Handle.Exec("INSERT INTO frontend_connection (time, frontend_id, connected, reason) VALUES (?, ?, ?, ?)", t, frontendID, up, reason)
	if err != nil {
		be.Logf(LogLevelNormal, "error saving connection history: %v\n", err)
	}
}

// closeOpenConnections ends the connections still marked up from before we
// started: we stopped without recording them ending, probably a crash. They
// end at the last stats sample taken while they were up, which is within one
// maintenance run of when we stopped.
func closeOpenConnections() error {
	rows, err := db.Handle.Query(`
		SELECT c.frontend_id, MAX(c.time, COALESCE((SELECT MAX(s.time) FROM server_sample AS s WHERE s.frontend_id = c.frontend_id), 0))
		FROM frontend_connection AS c
		WHERE c.connected = 1 AND c.id IN (SELECT MAX(id) FROM frontend_connection GROUP BY frontend_id)`)
	if err != nil {
		return fmt.Errorf("reading open connections: %v", err)
	}
	type open struct {
		id   int
		last int64
	}
	var opens []open
	for rows.Next() {
		var o open
		if err := rows.Scan(&o.id, &o.last); err != nil {
			rows.Close()
			return err
		}
		opens = append(opens, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, o := range opens {
		logConnectionAt(o.last, o.id, false, "CloudAdmin stopped")
	}
	return nil
}

// loadConnectionHistory reads fe's connection history from start until now.
// It returns nil if nothing is known about the span: no connection has ever
// been recorded up to now.
func loadConnectionHistory(fe *frontend.Frontend, start int64) (*ConnectionHistory, error) {
	fe.Mu.Lock()
	id := fe.ID
	fe.Mu.Unlock()
	h := &ConnectionHistory{Start: start, End: time.Now().Unix()}

	// the state going into the span
	var before int
	err := db.Handle.QueryRow("SELECT connected FROM frontend_connection WHERE frontend_id = ? AND time < ? ORDER BY time DESC, id DESC LIMIT 1", id, start).Scan(&before)
	known := err == nil
	if known {
		h.Points = append(h.Points, ConnPoint{start, before})
	}

	rows, err := db.Handle.Query("SELECT time, connected, reason FROM frontend_connection WHERE frontend_id = ? AND time >= ? AND time <= ? ORDER BY time, id", id, start, h.End)
	if err != nil {
		return nil, fmt.Errorf("reading connection history: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e ConnEvent
		if err := rows.Scan(&e.Time, &e.Connected, &e.Reason); err != nil {
			return nil, err
		}
		up := 0
		if e.Connected {
			up = 1
		} else {
			h.Drops++
		}
		h.Points = append(h.Points, ConnPoint{e.Time, up})
		h.Events = append(h.Events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(h.Points) == 0 {
		return nil, nil
	}
	last := h.Points[len(h.Points)-1]
	h.Points = append(h.Points, ConnPoint{h.End, last.Up}) // carry the state to now

	var upSecs int64
	for i := 1; i < len(h.Points); i++ {
		if h.Points[i-1].Up == 1 {
			upSecs += h.Points[i].T - h.Points[i-1].T
		}
	}
	if span := h.End - h.Points[0].T; span > 0 {
		h.Uptime = float64(upSecs) * 100 / float64(span)
	} else if last.Up == 1 {
		h.Uptime = 100
	}

	// newest first, most recent only
	for i, j := 0, len(h.Events)-1; i < j; i, j = i+1, j-1 {
		h.Events[i], h.Events[j] = h.Events[j], h.Events[i]
	}
	h.Events = h.Events[:min(len(h.Events), connectionEventsShown)]
	return h, nil
}

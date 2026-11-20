// Signals: q2admin scores players for things it finds undesirable or
// suspect (chat flooding, a VPN, aimbot-like aim and so on). Each signal it
// raises adds points, and a player whose points reach the server's threshold
// is removed. q2admin tells us about each change (CMDSignal), which is kept
// in the player_signal table and on the player while they're connected.
package backend

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/packetflinger/q2admind/frontend"
)

// What happened, in a CMDSignal (q2admin's cloud_signal_event_t)
const (
	SignalRaised  = iota // a signal started matching, or its points changed
	SignalCleared        // a signal stopped matching
	SignalRemoved        // the player's score reached the threshold, they're being removed
)

// signalsShown is how many signal events the server and recent player pages
// list.
const signalsShown = 50

// SignalEvent is one CMDSignal, as saved.
type SignalEvent struct {
	Time       int64
	Server     string // frontend name
	Client     int
	Event      int // Signal*
	Signal     int // q2admin's SIGNAL_* bit, 0 for a removal
	SignalName string
	Points     int
	Score      int
	Threshold  int
	Name       string
	IP         string
}

// ParseSignal handles a CMDSignal: q2admin raised or cleared one of a
// player's signals, or removed them for reaching the threshold. Signals can
// arrive before the player's CMDConnect, so the slot's signals are kept even
// while it's empty, and ParsePlayer carries them over. The message is:
//
//	byte    client slot
//	byte    event (Signal*)
//	long    signal bit, 0 for a removal
//	string  signal name, or for a removal every signal that counted
//	long    the signal's points now, 0 once cleared
//	long    the player's score afterwards
//	long    the server's threshold, 0 if off
//	string  player name
//	string  player IP
func ParseSignal(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	msg := &fe.Message
	ev := SignalEvent{
		Time:       time.Now().Unix(),
		Server:     fe.Name,
		Client:     msg.ReadByte(),
		Event:      msg.ReadByte(),
		Signal:     msg.ReadLong(),
		SignalName: msg.ReadString(),
		Points:     msg.ReadLong(),
		Score:      msg.ReadLong(),
		Threshold:  msg.ReadLong(),
		Name:       msg.ReadString(),
		IP:         msg.ReadString(),
	}
	fe.SignalLimit = ev.Threshold
	if fe.ValidPlayerID(ev.Client) {
		p := &fe.Players[ev.Client]
		if p.Signals == nil {
			p.Signals = map[string]int{}
		}
		switch ev.Event {
		case SignalRaised:
			p.Signals[ev.SignalName] = ev.Points
		case SignalCleared:
			delete(p.Signals, ev.SignalName)
		}
		p.SignalScore = ev.Score
	}
	logSignal(fe, ev)

	text := signalText(ev)
	line := func(name string) string {
		return fmt.Sprintf("%-20s[%d] %s %s", signalLabel(ev.Event), ev.Client, name, text)
	}
	fe.Log.Printf("%s", line(fmt.Sprintf("%q", ev.Name)))
	name := `"` + q2Text(ev.Name, ColorDefault) + `"`
	if p, err := fe.FindPlayer(ev.Client); err == nil {
		name = playerLink(fe, p, name, 0)
	}
	color := ColorYellow
	if ev.Event == SignalRemoved {
		color = ColorBrightRed
	}
	fe.ConsoleBroadcast(Font([]int{color}, line(name)))
}

// signalLabel is how the console and log name an event.
func signalLabel(event int) string {
	switch event {
	case SignalCleared:
		return "SIGNAL_CLEARED:"
	case SignalRemoved:
		return "SIGNAL_REMOVED:"
	}
	return "SIGNAL:"
}

// signalText describes an event after the player's name, like
// "chatflood +15 (score 35/50)".
func signalText(ev SignalEvent) string {
	score := fmt.Sprintf("(score %d)", ev.Score)
	if ev.Threshold > 0 {
		score = fmt.Sprintf("(score %d/%d)", ev.Score, ev.Threshold)
	}
	switch ev.Event {
	case SignalRaised:
		return fmt.Sprintf("%s %+d %s", ev.SignalName, ev.Points, score)
	case SignalCleared:
		return fmt.Sprintf("%s %s", ev.SignalName, score)
	}
	return fmt.Sprintf("%s: %s", score, ev.SignalName)
}

func logSignal(fe *frontend.Frontend, ev SignalEvent) {
	if db.Handle == nil {
		return
	}
	_, err := db.Handle.Exec(`INSERT INTO player_signal
		(time, frontend_id, client, event, signal, signal_name, points, score, threshold, name, ip)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.Time, fe.ID, ev.Client, ev.Event, ev.Signal, ev.SignalName, ev.Points, ev.Score, ev.Threshold, ev.Name, ev.IP)
	if err != nil {
		be.Logf(LogLevelNormal, "[%s] error saving signal: %v\n", fe.Name, err)
	}
}

// loadSignals returns up to limit signal events on fes, newest first, only
// the named player's if name isn't empty. It locks each frontend briefly, so
// call it before rendering, not during.
func loadSignals(fes []*frontend.Frontend, name string, limit int) ([]SignalEvent, error) {
	if len(fes) == 0 {
		return nil, nil
	}
	names := map[int]string{}
	var ids []any
	for _, fe := range fes {
		fe.Mu.Lock()
		ids = append(ids, fe.ID)
		names[fe.ID] = fe.Name
		fe.Mu.Unlock()
	}
	where := "frontend_id IN " + sqlIn(len(ids))
	args := ids
	if name != "" {
		where += " AND name = ?"
		args = append(args, name)
	}
	rows, err := db.Handle.Query(`SELECT time, frontend_id, client, event, signal, signal_name, points, score, threshold, name, ip
		FROM player_signal WHERE `+where+` ORDER BY time DESC, id DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("reading signals: %v", err)
	}
	defer rows.Close()
	var out []SignalEvent
	for rows.Next() {
		var ev SignalEvent
		var id int
		if err := rows.Scan(&ev.Time, &id, &ev.Client, &ev.Event, &ev.Signal, &ev.SignalName, &ev.Points, &ev.Score, &ev.Threshold, &ev.Name, &ev.IP); err != nil {
			return nil, err
		}
		ev.Server = names[id]
		out = append(out, ev)
	}
	return out, rows.Err()
}

// signalList is a player's current signals for display, highest points
// first, like "vpn-detected +40, chatflood +15".
func signalList(p *frontend.Player) string {
	type sig struct {
		name   string
		points int
	}
	var sigs []sig
	for n, pts := range p.Signals {
		sigs = append(sigs, sig{n, pts})
	}
	slices.SortFunc(sigs, func(a, b sig) int { return cmp.Or(cmp.Compare(b.points, a.points), cmp.Compare(a.name, b.name)) })
	var parts []string
	for _, s := range sigs {
		parts = append(parts, fmt.Sprintf("%s %+d", s.name, s.points))
	}
	return strings.Join(parts, ", ")
}

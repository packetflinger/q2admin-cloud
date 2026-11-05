// Teleport and invite history: each use of either in-game command is saved
// (see the teleport and invite tables), and the server page charts them.
package backend

import (
	"fmt"
	"time"

	"github.com/packetflinger/q2admind/frontend"
)

// CommandStats is one server's teleport and invite use over a stats span,
// bucketed like the rest of its Stats.
type CommandStats struct {
	Teleports    []int   // teleport commands used per bucket, listings included
	Invites      []int   // invites sent per bucket
	TeleportUses int     // teleport commands used, listings included
	Teleported   int     // players actually sent to another server
	InviteCount  int     // invites sent
	Destinations []Slice // servers teleported to, most first
	Inviters     []Slice // players who sent invites, most first
}

// logTeleport saves a use of the teleport command. args is what the player
// typed after it, dest the server it sent them to (empty if none).
func logTeleport(fe *frontend.Frontend, p *frontend.Player, args, dest string) {
	logCommand(fe, "INSERT INTO teleport (time, frontend_id, name, ip, args, destination) VALUES (?, ?, ?, ?, ?, ?)",
		time.Now().Unix(), fe.ID, p.Name, p.IP, args, dest)
}

// logInvite saves an invite a player sent, with its message.
func logInvite(fe *frontend.Frontend, p *frontend.Player, text string) {
	logCommand(fe, "INSERT INTO invite (time, frontend_id, name, ip, args) VALUES (?, ?, ?, ?, ?)",
		time.Now().Unix(), fe.ID, p.Name, p.IP, text)
}

func logCommand(fe *frontend.Frontend, query string, args ...any) {
	if fe == nil || db.Handle == nil {
		return
	}
	if _, err := db.Handle.Exec(query, args...); err != nil {
		be.Logf(LogLevelNormal, "[%s] error saving command history: %v\n", fe.Name, err)
	}
}

// loadCommandStats reads fe's teleport and invite history for st's span.
func loadCommandStats(st *Stats, fe *frontend.Frontend) (*CommandStats, error) {
	n := len(st.Times)
	cs := &CommandStats{Teleports: make([]int, n), Invites: make([]int, n)}
	if n == 0 {
		return cs, nil
	}
	start, end := st.Times[0], time.Now().Unix()
	width := int64(1)
	if n > 1 {
		width = st.Times[1] - st.Times[0]
	}
	bucket := func(t int64) int { return max(0, min(int((t-start)/width), n-1)) }
	fe.Mu.Lock()
	id := fe.ID
	fe.Mu.Unlock()

	rows, err := db.Handle.Query("SELECT time, destination FROM teleport WHERE frontend_id = ? AND time >= ? AND time <= ?", id, start, end)
	if err != nil {
		return nil, fmt.Errorf("reading teleports: %v", err)
	}
	dests := map[string]int{}
	for rows.Next() {
		var t int64
		var dest string
		if err := rows.Scan(&t, &dest); err != nil {
			rows.Close()
			return nil, err
		}
		cs.Teleports[bucket(t)]++
		cs.TeleportUses++
		if dest != "" {
			dests[dest]++
			cs.Teleported++
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	cs.Destinations = pie(dests)

	rows, err = db.Handle.Query("SELECT time, name FROM invite WHERE frontend_id = ? AND time >= ? AND time <= ?", id, start, end)
	if err != nil {
		return nil, fmt.Errorf("reading invites: %v", err)
	}
	defer rows.Close()
	inviters := map[string]int{}
	for rows.Next() {
		var t int64
		var name string
		if err := rows.Scan(&t, &name); err != nil {
			return nil, err
		}
		cs.Invites[bucket(t)]++
		cs.InviteCount++
		inviters[name]++
	}
	cs.Inviters = pie(inviters)
	return cs, rows.Err()
}

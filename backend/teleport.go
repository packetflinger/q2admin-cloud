// Teleporting is a feature where players can issue a console command (default
// is `!teleport` (configurable in the game library config)) to see what other
// frontends are available with info about current map and players. They can
// also join those frontends using the same command. The cloud admin server
// will stuff a connect message to the appropriate ip:port to the player.
package backend

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"text/template"
	"time"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

const (
	teleportTemplate = `
Teleporting between servers
Active servers:
  name                  map          players
  --------------------- ------------ ----------------------------------------
{{ range .ActiveServers }}  {{ printf "%-21s" .GetName }} {{ printf "%-12s" .GetMap }} {{ wrap .GetPlayers playersColumn }}
{{ end }}
Empty servers:
  {{ wrap .GetEmptyServers 2 }}
`

	// teleportPlayersColumn is where the players column starts in
	// teleportTemplate: the indent, then the name and map columns.
	teleportPlayersColumn = 2 + 21 + 1 + 12 + 1

	// MaxPrintLength is the most text sent to a player in one print. Quake 2
	// formats a print into a 1024-byte buffer (MAX_STRING_CHARS), so longer
	// output is sent as several prints.
	MaxPrintLength = 1000
)

var teleTmpl = template.Must(template.New("teleout").Funcs(template.FuncMap{
	"wrap":          wrapList,
	"playersColumn": func() int { return teleportPlayersColumn },
}).Parse(teleportTemplate))

// wrapList wraps a ", "-separated list so no line goes past TeleportWidth when
// it starts at column indent, breaking after a comma and indenting the lines
// it adds. An item too long for a line gets a line of its own.
func wrapList(list string, indent int) string {
	width := max(TeleportWidth-indent, 1)
	var lines []string
	line := ""
	for _, item := range strings.Split(list, ", ") {
		switch {
		case line == "":
			line = item
		case len(line)+len(", ")+len(item) > width:
			lines = append(lines, line+",")
			line = item
		default:
			line += ", " + item
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n"+strings.Repeat(" ", indent))
}

// chunkLines splits text into pieces of at most limit bytes, breaking between
// lines. A single line longer than limit is cut.
func chunkLines(text string, limit int) []string {
	var chunks []string
	chunk := ""
	for _, line := range strings.SplitAfter(text, "\n") {
		for len(line) > limit { // a line that can't fit in any chunk
			if chunk != "" {
				chunks = append(chunks, chunk)
				chunk = ""
			}
			chunks = append(chunks, line[:limit])
			line = line[limit:]
		}
		if len(chunk)+len(line) > limit {
			chunks = append(chunks, chunk)
			chunk = ""
		}
		chunk += line
	}
	if chunk != "" {
		chunks = append(chunks, chunk)
	}
	return chunks
}

// Build the proto to render using the output template.
func (s *Backend) teleportDestinations(self *frontend.Frontend) (*pb.TeleportReply, error) {
	var reply pb.TeleportReply
	var empty []string
	for _, fe := range s.Frontends() {
		// The caller holds its own frontend's lock. Waiting on another's could
		// deadlock with that server listing destinations at the same time.
		// shortcut: a server busy mid-packet is left out of this listing, cache
		// a per-server snapshot if that turns out to be noticeable.
		if fe != self {
			if !fe.Mu.TryLock() {
				continue
			}
			defer fe.Mu.Unlock()
		}
		if !fe.Trusted {
			continue
		}
		var dest pb.TeleportDestination
		dest.Name = fe.Name
		dest.Address = fmt.Sprintf("%s:%d", fe.IPAddress, fe.Port)
		dest.Map = fe.CurrentMap

		var players []string
		for _, p := range fe.Players {
			if p.Name != "" {
				players = append(players, p.Name)
			}
		}
		dest.Players = strings.Join(players, ", ")
		if dest.Players != "" {
			reply.ActiveServers = append(reply.ActiveServers, &dest)
		} else {
			empty = append(empty, fe.Name)
		}
	}
	reply.EmptyServers = strings.Join(empty, ", ")
	reply.ReplyDate = time.Now().Unix()
	return &reply, nil
}

// Teleport is called when a player issues the teleport command in-game. This
// command is configurable in the game library config (q2a_cloud.cfg). Just
// issuing the command will present the player with all the possible options
// for destination. Then issuing the command with the server name as an arg
// will cause the cloud admin server to stuff a connect command to the player.
func Teleport(fe *frontend.Frontend) {
	if fe == nil {
		log.Println("teleport problem: frontend was nil")
		return
	}
	sv := fe.Server.(*Backend)
	player := (&fe.Message).ReadByte()
	target := (&fe.Message).ReadString()
	p, err := fe.FindPlayer(int(player))
	if err != nil {
		sv.Logf(LogLevelInfo, "teleport error: %v\n", err)
		return
	}
	if !fe.AllowTeleport {
		SayPlayer(fe, p, PRINT_CHAT, "Teleporting is disabled on this server")
		return
	}
	if target == "" {
		dests, err := sv.teleportDestinations(fe)
		if err != nil {
			sv.Logf(LogLevelInfo, "error collecting teleport destinations: %v\n", err)
			return
		}
		var rendered bytes.Buffer
		if err := teleTmpl.Execute(&rendered, dests); err != nil {
			sv.Logf(LogLevelInfo, "error executing teleport template: %v\n", err)
		}
		for _, chunk := range chunkLines(rendered.String(), MaxPrintLength) {
			SayPlayer(fe, p, PRINT_CHAT, chunk)
		}
		return
	}

	for _, f := range sv.Frontends() {
		if strings.EqualFold(f.Name, target) {
			notice := fmt.Sprintf("Teleporting %s to %s [%s:%d]\n", p.Name, f.Name, f.IPAddress, f.Port)
			SayEveryone(fe, PRINT_CHAT, notice)

			cmd := fmt.Sprintf("connect %s:%d\n", f.IPAddress, f.Port)
			StuffPlayer(fe, p, cmd)
			// tell both servers' console watchers; the destination's console
			// only needs its own terminals lock, not f.Mu
			fe.ConsoleBroadcast(Font([]int{ColorBrightCyan}, fmt.Sprintf("%s [%d] teleported to %s", playerLink(fe, p, p.Name, 0), p.ClientID, consoleLink(f))))
			f.ConsoleBroadcast(Font([]int{ColorBrightCyan}, fmt.Sprintf("%s teleported from %s", p.Name, consoleLink(fe))))
			p.LastTeleport = time.Now().Unix()
			p.Teleports++
			f.TeleportCount++
			return
		}
	}
}

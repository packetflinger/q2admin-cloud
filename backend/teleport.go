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
  {{ if .ActiveServers }}{{ range .ActiveServers }}{{ printf "%-21s" .GetName}} {{printf "%-12s" .GetMap }} {{ .GetPlayers}}{{ end }}{{ end }} 

Empty servers:
  {{ .GetEmptyServers }}
`
)

// Build the proto to render using the output template.
func (s *Backend) teleportDestinations() (*pb.TeleportReply, error) {
	var reply pb.TeleportReply
	var empty []string
	for _, fe := range s.frontends {
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
	reply.EmptyServers = strings.Join(empty, ",")
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
		teleTmpl := template.Must(template.New("teleout").Parse(teleportTemplate))
		dests, err := sv.teleportDestinations()
		if err != nil {
			sv.Logf(LogLevelInfo, "error collecting teleport destinations: %v\n", err)
			return
		}
		var rendered bytes.Buffer
		if err := teleTmpl.Execute(&rendered, dests); err != nil {
			sv.Logf(LogLevelInfo, "error executing teleport template: %v\n", err)
		}
		SayPlayer(fe, p, PRINT_CHAT, rendered.String())
		return
	}

	for i, f := range sv.frontends {
		if strings.EqualFold(f.Name, target) {
			notice := fmt.Sprintf("Teleporting to %s to %s [%s:%d]\n", p.Name, f.Name, f.IPAddress, f.Port)
			SayEveryone(fe, PRINT_CHAT, notice)

			cmd := fmt.Sprintf("connect %s:%d\n", f.IPAddress, f.Port)
			StuffPlayer(fe, p, cmd)
			p.LastTeleport = time.Now().Unix()
			p.Teleports++
			sv.frontends[i].TeleportCount++
			return
		}
	}
}

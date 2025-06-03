// Teleporting is a feature where players can issue a console command (default
// is `!teleport` (configurable in the game library config)) to see what other
// servers are available with info about current map and players. They can also
// join those server using the same command. The cloud admin server will stuff
// a connect message to the appropriate ip:port to the player.
package server

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"text/template"
	"time"

	"github.com/packetflinger/q2admind/client"

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
func (s *Server) teleportDestinations() (*pb.TeleportReply, error) {
	var reply pb.TeleportReply
	var empty []string
	for _, cl := range s.clients {
		if !cl.Trusted {
			continue
		}
		var dest pb.TeleportDestination
		dest.Name = cl.Name
		dest.Address = fmt.Sprintf("%s:%d", cl.IPAddress, cl.Port)
		dest.Map = cl.CurrentMap

		var players []string
		for _, p := range cl.Players {
			if p.Name != "" {
				players = append(players, p.Name)
			}
		}
		dest.Players = strings.Join(players, ", ")
		if dest.Players != "" {
			reply.ActiveServers = append(reply.ActiveServers, &dest)
		} else {
			empty = append(empty, cl.Name)
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
func Teleport(cl *client.Client) {
	if cl == nil {
		log.Println("teleport problem: client was nil")
		return
	}
	sv := cl.Server.(*Server)
	player := (&cl.Message).ReadByte()
	target := (&cl.Message).ReadString()
	p, err := cl.FindPlayer(int(player))
	if err != nil {
		sv.Logf(LogLevelInfo, "teleport error: %v\n", err)
		return
	}
	if !cl.AllowTeleport {
		SayPlayer(cl, p, PRINT_CHAT, "Teleporting is disabled on this server")
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
		SayPlayer(cl, p, PRINT_CHAT, rendered.String())
		return
	}

	for i, c := range sv.clients {
		if strings.EqualFold(c.Name, target) {
			notice := fmt.Sprintf("Teleporting to %s to %s [%s:%d]\n", p.Name, c.Name, c.IPAddress, c.Port)
			SayEveryone(cl, PRINT_CHAT, notice)

			cmd := fmt.Sprintf("connect %s:%d\n", c.IPAddress, c.Port)
			StuffPlayer(cl, p, cmd)
			p.LastTeleport = time.Now().Unix()
			p.Teleports++
			sv.clients[i].TeleportCount++
			return
		}
	}
}

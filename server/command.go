package server

import (
	"errors"
	"fmt"
	"log"
	"sort"

	//"strings"
	"time"

	"github.com/packetflinger/q2admind/client"
)

/**
 * Player issued the teleport command.
 *
 * If a destination is supplied, just send the player there,
 * else send a list of possibilities
 */
func Teleport(cl *client.Client) {
	msg := &cl.Message
	pl := msg.ReadByte()
	dest := msg.ReadString()
	p := cl.FindPlayer(int(pl))

	now := time.Now().Unix()
	log.Printf("[%s/TELEPORT/%s] %s\n", p.Name, p.Name, dest)

	if dest == "" {
		listtime := now - p.LastTeleportList
		if listtime < 30 {
			txt := fmt.Sprintf("You can't list teleport destinations for %d more seconds\n", 30-listtime)
			cl.SayPlayer(p, PRINT_HIGH, txt)
			return
		}

		p.LastTeleportList = now
		avail := TeleportAvailableReply()
		cl.SayPlayer(p, PRINT_CHAT, avail)

		cl.SayPlayer(p, PRINT_CHAT, "Active Servers\n")
		line := ""

		for _, c := range Q2A.Clients {
			if len(c.Players) == 0 {
				continue
			}

			players := ""
			for _, p := range c.Players {
				players = fmt.Sprintf("%s %s", players, p.Name)
			}

			line = fmt.Sprintf(" %-15s %-15s %s\n", c.Name, c.CurrentMap, players)
			cl.SayPlayer(p, PRINT_CHAT, line)
		}
		return
	}

	s, err := FindTeleportDestination(dest)
	p.LastTeleport = now
	p.Teleports++

	if err != nil {
		log.Println("warning,", err)
		cl.SayPlayer(p, PRINT_HIGH, "Unknown destination\n")
	} else {
		txt := fmt.Sprintf("Teleporting %s to %s [%s:%d]\n", p.Name, s.Name, s.IPAddress, s.Port)
		cl.SayEveryone(PRINT_HIGH, txt)
		st := fmt.Sprintf("connect %s:%d\n", s.IPAddress, s.Port)
		StuffPlayer(cl, *p, st)
	}

	//txt := fmt.Sprintf("TELEPORT [%d] %s", pl, p.Name)
	//LogEventToDatabase(cl.ID, LogTypeCommand, txt)
}

// Resolve a teleport name to an ip:port
func FindTeleportDestination(dest string) (*client.Client, error) {
	for i, c := range Q2A.Clients {
		if c.Name == dest {
			return &Q2A.Clients[i], nil
		}
	}

	return nil, errors.New("unknown destination")
}

func TeleportAvailableReply() string {
	var allservers []string

	for _, c := range Q2A.Clients {
		if !c.Connected {
			continue
		}

		allservers = append(allservers, c.Name)
	}

	// alphabetize the list
	sort.Strings(allservers)

	serverstr := "Available Servers:"
	for _, s := range allservers {
		serverstr = fmt.Sprintf("%s %s", serverstr, s)
	}
	serverstr = fmt.Sprintf("%s\n", serverstr)

	return serverstr
}

/**
 * Player issued an invite command.
 *
 * Broadcast the invite to all connected servers
 */
func Invite(cl *client.Client) {
	client := (&cl.Message).ReadByte()
	text := (&cl.Message).ReadString()
	p := cl.FindPlayer(int(client))
	log.Printf("[%s/INVITE/%s] %s\n", cl.Name, p.Name, text)

	now := time.Now().Unix()
	invtime := now - p.LastInvite

	if p.InvitesAvailable == 0 {
		if invtime > 600 {
			p.InvitesAvailable = 3
		} else {
			txt := fmt.Sprintf("You have no more invites available, wait %d seconds\n", 600-invtime)
			cl.SayPlayer(p, PRINT_HIGH, txt)
			return
		}
	} else {
		if invtime < 30 {
			txt := fmt.Sprintf("Invite used too recently, wait %d seconds\n", 30-invtime)
			cl.SayPlayer(p, PRINT_HIGH, txt)
			return
		}
	}

	inv := fmt.Sprintf("%s invites you to play at %s (%s:%d)", p.Name, cl.Name, cl.IPAddress, cl.Port)
	for _, s := range Q2A.Clients {
		if s.Enabled && s.Connected {
			s.SayEveryone(PRINT_CHAT, inv)
		}
	}

	p.Invites++
	p.LastInvite = now
	p.InvitesAvailable--
}

// Have client broadcast print from "console"
func ConsoleSay(cl *client.Client, print string) {
	if print == "" {
		return
	}

	txt := fmt.Sprintf("say %s\n", print)
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(txt)
}

/**
 * Force a player to do a command
 */
func StuffPlayer(cl *client.Client, p client.Player, cmd string) {
	stuffcmd := fmt.Sprintf("sv !stuff CL %d %s\n", p.ClientID, cmd)
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(stuffcmd)
}

/**
 * Temporarily prevent the player from talking
 * using a negative number of seconds makes it
 * permanent.
 */
func (cl *Client) MutePlayer(p *Player, seconds int) {
	cmd := ""
	if seconds < 0 {
		cmd = fmt.Sprintf("sv !mute CL %d PERM\n", p.ClientID)
	} else {
		cmd = fmt.Sprintf("sv !mute CL %d %d", p.ClientID, seconds)
	}
	WriteByte(SCMDCommand, &cl.MessageOut)
	WriteString(cmd, &cl.MessageOut)
	player := cl.FindPlayer(p.ClientID)

	txt := fmt.Sprintf("[%s/MUTE] %d|%s was muted", cl.Name, p.ClientID, player.Name)
	LogEventToDatabase(cl.ID, LogTypeCommand, txt)
}

/**
 *
 */
func (cl *Client) KickPlayer(p *Player, msg string) {
	cmd := fmt.Sprintf("kick %d\n", p.ClientID)
	WriteByte(SCMDCommand, &cl.MessageOut)
	WriteString(cmd, &cl.MessageOut)

	txt := fmt.Sprintf("KICK [%d] was kicked", p.ClientID)
	LogEventToDatabase(cl.ID, LogTypeCommand, txt)
}

// Issue a command as if you were typing it into the console.
// Sanitize cmd before use
func (cl *Client) ConsoleCommand(cmd string) {
	WriteByte(SCMDCommand, &cl.MessageOut)
	WriteString(cmd, &cl.MessageOut)
}

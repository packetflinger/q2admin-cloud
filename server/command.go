package server

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	//"strings"
	"time"

	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/crypto"
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

	p, err := cl.FindPlayer(int(pl))
	if err != nil {
		cl.Log.Println("teleport problem:", err)
		cl.SSHPrintln("* teleport problem: " + err.Error())
	}

	now := time.Now().Unix()
	log.Printf("[%s/TELEPORT/%s] %s\n", p.Name, p.Name, dest)

	if dest == "" {
		listtime := now - p.LastTeleportList
		if listtime < 30 {
			txt := fmt.Sprintf("You can't list teleport destinations for %d more seconds\n", 30-listtime)
			SayPlayer(cl, p, PRINT_HIGH, txt)
			return
		}

		p.LastTeleportList = now
		avail := TeleportAvailableReply()
		SayPlayer(cl, p, PRINT_CHAT, avail)

		SayPlayer(cl, p, PRINT_CHAT, "Active Servers\n")
		line := ""

		for _, c := range srv.clients {
			if len(c.Players) == 0 {
				continue
			}

			players := ""
			for _, p := range c.Players {
				players = fmt.Sprintf("%s %s", players, p.Name)
			}

			line = fmt.Sprintf(" %-15s %-15s %s\n", c.Name, c.CurrentMap, players)
			SayPlayer(cl, p, PRINT_CHAT, line)
		}
		return
	}

	s, err := FindTeleportDestination(dest)
	p.LastTeleport = now
	p.Teleports++

	if err != nil {
		log.Println("warning,", err)
		SayPlayer(cl, p, PRINT_HIGH, "Unknown destination\n")
	} else {
		txt := fmt.Sprintf("Teleporting %s to %s [%s:%d]\n", p.Name, s.Name, s.IPAddress, s.Port)
		SayEveryone(cl, PRINT_HIGH, txt)
		st := fmt.Sprintf("connect %s:%d\n", s.IPAddress, s.Port)
		StuffPlayer(cl, *p, st)
	}

	//txt := fmt.Sprintf("TELEPORT [%d] %s", pl, p.Name)
	//LogEventToDatabase(cl.ID, LogTypeCommand, txt)
}

// Resolve a teleport name to an ip:port
func FindTeleportDestination(dest string) (*client.Client, error) {
	for i, c := range srv.clients {
		if c.Name == dest {
			return &srv.clients[i], nil
		}
	}

	return nil, errors.New("unknown destination")
}

func TeleportAvailableReply() string {
	var allservers []string

	for _, c := range srv.clients {
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

	p, err := cl.FindPlayer(int(client))
	if err != nil {
		cl.Log.Println("invite problem:", err)
		cl.SSHPrintln("invite problem: " + err.Error())
	}
	log.Printf("[%s/INVITE/%s] %s\n", cl.Name, p.Name, text)

	now := time.Now().Unix()
	invtime := now - p.LastInvite

	if p.InvitesAvailable == 0 {
		if invtime > 600 {
			p.InvitesAvailable = 3
		} else {
			txt := fmt.Sprintf("You have no more invites available, wait %d seconds\n", 600-invtime)
			SayPlayer(cl, p, PRINT_HIGH, txt)
			return
		}
	} else {
		if invtime < 30 {
			txt := fmt.Sprintf("Invite used too recently, wait %d seconds\n", 30-invtime)
			SayPlayer(cl, p, PRINT_HIGH, txt)
			return
		}
	}

	inv := fmt.Sprintf("%s invites you to play at %s (%s:%d)", p.Name, cl.Name, cl.IPAddress, cl.Port)
	for _, s := range srv.clients {
		if s.Enabled && s.Connected {
			SayEveryone(&s, PRINT_CHAT, inv)
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
	SendMessages(cl)
}

// Force a player to do a command
func StuffPlayer(cl *client.Client, p client.Player, cmd string) {
	stuffcmd := fmt.Sprintf("sv !stuff CL %d %s\n", p.ClientID, cmd)
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(stuffcmd)
	SendMessages(cl)
}

// Prevent the player from talking.
// Using a negative number of seconds makes it permanent (mute vs stifle).
func MutePlayer(cl *client.Client, p *client.Player, seconds int) {
	var cmd string
	if seconds < 0 {
		cmd = fmt.Sprintf("sv !mute CL %d PERM\n", p.ClientID)
		cl.Log.Printf("MUTE %s\\%d\n", p.Name, p.ClientID)
	} else {
		cmd = fmt.Sprintf("sv !mute CL %d %d", p.ClientID, seconds)
		cl.Log.Printf("STIFLE[%d] %s\\%d\n", p.StifleLength, p.Name, p.ClientID)
	}
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(cmd)
	SendMessages(cl)
}

// Tell the client to disconnect a specific player
func KickPlayer(cl *client.Client, p *client.Player, msg string) {
	cmd := fmt.Sprintf("kick %d\n", p.ClientID)
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(cmd)
	SendMessages(cl)

	//txt := fmt.Sprintf("KICK [%d] was kicked", p.ClientID)
	//LogEventToDatabase(cl.ID, LogTypeCommand, txt)
}

// Issue a command as if you were typing it into the console.
//
// TODO: Sanitize cmd before use
func ConsoleCommand(cl *client.Client, cmd string) {
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(cmd)
	SendMessages(cl)
}

// Send a message to every player on the server
func SayEveryone(cl *client.Client, level int, text string) {
	if text == "" {
		return
	}
	(&cl.MessageOut).WriteByte(SCMDSayAll)
	(&cl.MessageOut).WriteByte(byte(level))
	(&cl.MessageOut).WriteString(text)
	SendMessages(cl)
}

// Send a message to a particular player
func SayPlayer(cl *client.Client, p *client.Player, level int, text string) {
	if text == "" {
		return
	}

	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	msg := &cl.MessageOut
	msg.WriteByte(SCMDSayClient)
	msg.WriteByte(byte(p.ClientID))
	msg.WriteByte(byte(level))
	msg.WriteString(text)
	SendMessages(cl)
}

// Setup a new cookie on a player
//
// Player cookies are a dirty and not terribly effective way of
// uniquely identifying players. Original Q2 made no effort to
// ID players other than their client number. Every player in the
// game can have the same name, skin, etc. The player's IP address
// was the only way to really differentiate them from other players.
// Now in the age of VPNs, an malicious player can get banned, and
// reconnect on a VPN with different IP, different name, etc, and
// continue being abusive.
//
// The idea of a player cookie is a persistent unique identifier.
// PlayerX can reconnect with a different name on a different IP
// with a different client and still be identified. This is great
// for tracking statistics and disciplinary actions (muting/banning
// shitheads).
func SetupPlayerCookie(cl *client.Client, p *client.Player) {
	value := hex.EncodeToString(crypto.RandomBytes(12)) // random ID

	// "modern" clients (q2pro, r1q2) support seta for archive vars
	a := fmt.Sprintf("seta cl_cookie %s", value)

	// ancient clients (3.2[01]) require old format "set name value a"
	//a_old := fmt.Sprintf("set cl_cookie %s a", value)

	u := "setu cl_cookie $cl_cookie"

	// tell player to write the var to local .cfg file for persistence
	StuffPlayer(cl, *p, a)

	// tell player to add var to their userinfo string. This will
	// trigger a ClientUserinfoChanged() call on the game server
	StuffPlayer(cl, *p, u)
}

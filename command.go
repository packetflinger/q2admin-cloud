package main

import (
	"errors"
	"fmt"
	"log"
	"sort"

	//"strings"
	"time"
)

/**
 * Player issued the teleport command.
 *
 * If a destination is supplied, just send the player there,
 * else send a list of possibilities
 */
func Teleport(srv *Server) {
	cl := ReadByte(&srv.Message)
	dest := ReadString(&srv.Message)
	p := srv.FindPlayer(int(cl))

	now := time.Now().Unix()
	log.Printf("[%s/TELEPORT/%s] %s\n", srv.Name, p.Name, dest)

	if dest == "" {
		listtime := now - p.LastTeleportList
		if listtime < 30 {
			txt := fmt.Sprintf("You can't list teleport destinations for %d more seconds\n", 30-listtime)
			srv.SayPlayer(int(cl), PRINT_HIGH, txt)
			return
		}

		p.LastTeleportList = now
		avail := TeleportAvailableReply()
		srv.SayPlayer(int(cl), PRINT_CHAT, avail)

		srv.SayPlayer(int(cl), PRINT_CHAT, "Active Servers\n")
		line := ""

		for _, s := range servers {
			if len(s.Players) == 0 {
				continue
			}

			players := ""
			for _, p := range s.Players {
				players = fmt.Sprintf("%s %s", players, p.Name)
			}

			line = fmt.Sprintf(" %-15s %-15s %s\n", s.Name, s.CurrentMap, players)
			srv.SayPlayer(int(cl), PRINT_CHAT, line)
		}
		return
	}

	s, err := FindTeleportDestination(dest)
	p.LastTeleport = now
	p.Teleports++

	if err != nil {
		log.Println("warning,", err)
		srv.SayPlayer(int(cl), PRINT_HIGH, "Unknown destination\n")
	} else {
		txt := fmt.Sprintf("Teleporting %s to %s [%s:%d]\n", p.Name, s.Name, s.IPAddress, s.Port)
		srv.SayEveryone(PRINT_HIGH, txt)
		st := fmt.Sprintf("connect %s:%d\n", s.IPAddress, s.Port)
		StuffPlayer(srv, int(cl), st)
	}

	txt := fmt.Sprintf("TELEPORT [%d] %s", cl, p.Name)
	LogEventToDatabase(srv.ID, LogTypeCommand, txt)
}

/**
 * Resolve a teleport name to an ip:port
 */
func FindTeleportDestination(dest string) (*Server, error) {
	for _, s := range servers {
		if s.Name == dest {
			return &s, nil
		}
	}

	return nil, errors.New("unknown destination")
}

func TeleportAvailableReply() string {
	var allservers []string

	for _, s := range servers {
		if !s.Connected {
			continue
		}

		allservers = append(allservers, s.Name)
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
func Invite(srv *Server) {
	cl := ReadByte(&srv.Message)
	text := ReadString(&srv.Message)
	p := srv.FindPlayer(int(cl))
	log.Printf("[%s/INVITE/%s] %s\n", srv.Name, p.Name, text)

	now := time.Now().Unix()
	invtime := now - p.LastInvite

	if p.InvitesAvailable == 0 {
		if invtime > 600 {
			p.InvitesAvailable = 3
		} else {
			txt := fmt.Sprintf("You have no more invites available, wait %d seconds\n", 600-invtime)
			srv.SayPlayer(int(cl), PRINT_HIGH, txt)
			return
		}
	} else {
		if invtime < 30 {
			txt := fmt.Sprintf("Invite used too recently, wait %d seconds\n", 30-invtime)
			srv.SayPlayer(int(cl), PRINT_HIGH, txt)
			return
		}
	}

	inv := fmt.Sprintf("%s invites you to play at %s (%s:%d)", p.Name, srv.Name, srv.IPAddress, srv.Port)
	for _, s := range servers {
		if s.Enabled && s.Connected {
			s.SayEveryone(PRINT_CHAT, inv)
		}
	}

	p.Invites++
	p.LastInvite = now
	p.InvitesAvailable--
}

func ConsoleSay(srv *Server, print string) {
	if print == "" {
		return
	}

	txt := fmt.Sprintf("say %s\n", print)
	WriteByte(SCMDCommand, &srv.MessageOut)
	WriteString(txt, &srv.MessageOut)
}

/**
 * Force a player to do a command
 */
func StuffPlayer(srv *Server, cl int, cmd string) {
	stuffcmd := fmt.Sprintf("sv !stuff CL %d %s\n", cl, cmd)
	WriteByte(SCMDCommand, &srv.MessageOut)
	WriteString(stuffcmd, &srv.MessageOut)
}

/**
 * Temporarily prevent the player from talking
 * using a negative number of seconds makes it
 * permanent.
 */
func MutePlayer(srv *Server, cl int, seconds int) {
	cmd := ""
	if seconds < 0 {
		cmd = fmt.Sprintf("sv !mute CL %d PERM\n", cl)
	} else {
		cmd = fmt.Sprintf("sv !mute CL %d %d", cl, seconds)
	}
	WriteByte(SCMDCommand, &srv.MessageOut)
	WriteString(cmd, &srv.MessageOut)
	player := srv.FindPlayer(cl)

	txt := fmt.Sprintf("[%s/MUTE] %d|%s was muted", srv.Name, cl, player.Name)
	LogEventToDatabase(srv.ID, LogTypeCommand, txt)
}

/**
 *
 */
func KickPlayer(srv *Server, cl int) {
	cmd := fmt.Sprintf("kick %d", cl)
	WriteByte(SCMDCommand, &srv.MessageOut)
	WriteString(cmd, &srv.MessageOut)

	txt := fmt.Sprintf("KICK [%d] was kicked", cl)
	LogEventToDatabase(srv.ID, LogTypeCommand, txt)
}

//
// Issue a command as if you were typing it into the console.
// Sanitize cmd before use
//
func (srv *Server) ConsoleCommand(cmd string) {
	WriteByte(SCMDCommand, &srv.MessageOut)
	WriteString(cmd, &srv.MessageOut)
}

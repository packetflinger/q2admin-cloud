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
	cl := ReadByte(&srv.message)
	dest := ReadString(&srv.message)
	p := srv.FindPlayer(int(cl))

	now := time.Now().Unix()
	log.Printf("[%s/TELEPORT/%s] %s\n", srv.name, p.name, dest)

	if dest == "" {
		listtime := now - p.lastteleportlist
		if listtime < 30 {
			txt := fmt.Sprintf("You can't list teleport destinations for %d more seconds\n", 30-listtime)
			SayPlayer(srv, int(cl), PRINT_HIGH, txt)
			return
		}

		p.lastteleportlist = now
		avail := TeleportAvailableReply()
		SayPlayer(srv, int(cl), PRINT_CHAT, avail)

		SayPlayer(srv, int(cl), PRINT_CHAT, "Active Servers\n")
		line := ""

		for _, s := range servers {
			if len(s.players) == 0 {
				continue
			}

			players := ""
			for _, p := range s.players {
				players = fmt.Sprintf("%s %s", players, p.name)
			}

			line = fmt.Sprintf(" %-15s %-15s %s\n", s.name, s.currentmap, players)
			SayPlayer(srv, int(cl), PRINT_CHAT, line)
		}
		return
	}

	s, err := FindTeleportDestination(dest)
	p.lastteleport = now
	p.teleports++

	if err != nil {
		log.Println("warning,", err)
		SayPlayer(srv, int(cl), PRINT_HIGH, "Unknown destination\n")
	} else {
		txt := fmt.Sprintf("Teleporting %s to %s [%s:%d]\n", p.name, s.name, s.ipaddress, s.port)
		srv.SayEveryone(PRINT_HIGH, txt)
		st := fmt.Sprintf("connect %s:%d\n", s.ipaddress, s.port)
		StuffPlayer(srv, int(cl), st)
	}

	txt := fmt.Sprintf("TELEPORT [%d] %s", cl, p.name)
	LogEventToDatabase(srv.id, LogTypeCommand, txt)
}

/**
 * Resolve a teleport name to an ip:port
 */
func FindTeleportDestination(dest string) (*Server, error) {
	for _, s := range servers {
		if s.name == dest {
			return &s, nil
		}
	}

	return nil, errors.New("unknown destination")
}

func TeleportAvailableReply() string {
	var allservers []string

	for _, s := range servers {
		if !s.connected {
			continue
		}

		allservers = append(allservers, s.name)
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
	cl := ReadByte(&srv.message)
	text := ReadString(&srv.message)
	p := srv.FindPlayer(int(cl))
	log.Printf("[%s/INVITE/%s] %s\n", srv.name, p.name, text)

	now := time.Now().Unix()
	invtime := now - p.lastinvite

	if p.invitesavailable == 0 {
		if invtime > 600 {
			p.invitesavailable = 3
		} else {
			txt := fmt.Sprintf("You have no more invites available, wait %d seconds\n", 600-invtime)
			SayPlayer(srv, int(cl), PRINT_HIGH, txt)
			return
		}
	} else {
		if invtime < 30 {
			txt := fmt.Sprintf("Invite used too recently, wait %d seconds\n", 30-invtime)
			SayPlayer(srv, int(cl), PRINT_HIGH, txt)
			return
		}
	}

	inv := fmt.Sprintf("%s invites you to play at %s (%s:%d)", p.name, srv.name, srv.ipaddress, srv.port)
	for _, s := range servers {
		if s.enabled && s.connected {
			s.SayEveryone(PRINT_CHAT, inv)
		}
	}

	p.invites++
	p.lastinvite = now
	p.invitesavailable--
}

func ConsoleSay(srv *Server, print string) {
	if print == "" {
		return
	}

	txt := fmt.Sprintf("say %s\n", print)
	WriteByte(SCMDCommand, &srv.messageout)
	WriteString(txt, &srv.messageout)
}

/**
 * Force a player to do a command
 */
func StuffPlayer(srv *Server, cl int, cmd string) {
	stuffcmd := fmt.Sprintf("sv !stuff CL %d %s\n", cl, cmd)
	WriteByte(SCMDCommand, &srv.messageout)
	WriteString(stuffcmd, &srv.messageout)
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
	WriteByte(SCMDCommand, &srv.messageout)
	WriteString(cmd, &srv.messageout)
	player := srv.FindPlayer(cl)

	txt := fmt.Sprintf("[%s/MUTE] %d|%s was muted", srv.name, cl, player.name)
	LogEventToDatabase(srv.id, LogTypeCommand, txt)
}

/**
 *
 */
func KickPlayer(srv *Server, cl int) {
	cmd := fmt.Sprintf("kick %d", cl)
	WriteByte(SCMDCommand, &srv.messageout)
	WriteString(cmd, &srv.messageout)

	txt := fmt.Sprintf("KICK [%d] was kicked", cl)
	LogEventToDatabase(srv.id, LogTypeCommand, txt)
}

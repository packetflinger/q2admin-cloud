package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
)

/**
 * Loop through all the data from the client
 * and act accordingly
 */
func (srv *Server) ParseMessage() {
	msg := &srv.Message
	for {
		if msg.index >= len(msg.buffer) {
			break
		}

		switch b := ReadByte(msg); b {
		case CMDPing:
			Pong(srv)

		case CMDPrint:
			ParsePrint(srv)

		case CMDMap:
			ParseMap(srv)

		case CMDPlayerList:
			ParsePlayerlist(srv)

		case CMDConnect:
			ParseConnect(srv)

		case CMDDisconnect:
			ParseDisconnect(srv)

		case CMDCommand:
			ParseCommand(srv)

		case CMDFrag:
			ParseFrag(srv)
		}
	}
}

/**
 * A player was fragged.
 * Only two bytes are sent: the clientID of the victim,
 * and of the attacker
 */
func ParseFrag(srv *Server) {
	v := int(ReadByte(&srv.Message))
	a := int(ReadByte(&srv.Message))

	victim := srv.FindPlayer(v)
	attacker := srv.FindPlayer(a)

	if victim == nil {
		return
	}

	log.Printf("[%s/FRAG] %d > %d\n", srv.Name, a, v)

	if attacker == victim || attacker == nil {
		victim.suicides++
		victim.frags--
		victim.deaths++
	} else {
		attacker.frags++
		victim.deaths++
	}
}

/**
 * Received a ping from a client, send a pong to show we're alive
 */
func Pong(srv *Server) {
	if config.Debug > 1 {
		log.Printf("[%s/PING]\n", srv.Name)
	}
	srv.PingCount++
	WriteByte(SCMDPong, &srv.MessageOut)

	// close to once per hour
	if (srv.PingCount & 63) == 0 {
		RotateKeys(srv)
	}
}

/**
 * A print was sent by the server.
 * 1 byte: print level
 * string: the actual message
 */
func ParsePrint(srv *Server) {
	level := ReadByte(&srv.Message)
	text := ReadString(&srv.Message)

	switch level {
	case PRINT_CHAT:
		LogChat(srv, text)
		log.Printf("[%s/PRINT] (%d) %s\n", srv.Name, level, text)
	case PRINT_MEDIUM:
		ParseObituary(text)
	}
}

/**
 * A player connected to the a q2 server
 */
func ParseConnect(srv *Server) {
	p := ParsePlayer(srv)

	if p == nil {
		return
	}

	info := UserinfoMap(p.userinfo)

	txt := fmt.Sprintf("[%s/CONNECT] %d|%s|%s|%s", srv.Name, p.clientid, info["name"], info["ip"], p.hash)
	log.Printf("%s\n", txt)
	LogEventToDatabase(srv.ID, LogTypeJoin, txt)

	// global
	if isbanned, msg := CheckForBan(&globalbans, p.ip); isbanned == Banned {
		srv.SayPlayer(
			p.clientid,
			PRINT_CHAT,
			fmt.Sprintf("Your IP/Userinfo matches a global ban: %s\n", msg),
		)
		KickPlayer(srv, p.clientid)
		return
	}

	// local
	if isbanned, msg := CheckForBan(&srv.Bans, p.ip); isbanned == Banned {
		srv.SayPlayer(
			p.clientid,
			PRINT_CHAT,
			fmt.Sprintf("Your IP/Userinfo matches a local ban: %s\n", msg),
		)
		KickPlayer(srv, p.clientid)
	}
}

/**
 * A player disconnected from a q2 server
 */
func ParseDisconnect(srv *Server) {
	clientnum := int(ReadByte(&srv.Message))

	if clientnum < 0 || clientnum > srv.MaxPlayers {
		log.Printf("Invalid client number: %d\n%s\n", clientnum, hex.Dump(srv.Message.buffer))
		return
	}

	pl := srv.FindPlayer(clientnum)
	log.Printf("[%s/DISCONNECT] %d|%s\n", srv.Name, clientnum, pl.name)
	srv.RemovePlayer(clientnum)
}

/**
 * Server told us what map is currently running. Typically happens
 * when the map changes
 */
func ParseMap(srv *Server) {
	mapname := ReadString(&srv.Message)
	srv.CurrentMap = mapname
	log.Printf("[%s/MAP] %s\n", srv.Name, srv.CurrentMap)
}

func ParseObituary(text string) {
	log.Printf("Obit: %s\n", text)
}

func ParsePlayerlist(srv *Server) {
	count := ReadByte(&srv.Message)
	log.Printf("[%s/PLAYERLIST] %d\n", srv.Name, count)
	for i := 0; i < int(count); i++ {
		_ = ParsePlayer(srv)
	}
}

func ParsePlayer(srv *Server) *Player {
	clientnum := ReadByte(&srv.Message)
	userinfo := ReadString(&srv.Message)

	if int(clientnum) > srv.MaxPlayers {
		log.Printf("WARNING: Invalid client number, ignoring\n")
		return nil
	}

	info := UserinfoMap(userinfo)
	port, _ := strconv.Atoi(info["port"])
	fov, _ := strconv.Atoi(info["fov"])
	newplayer := Player{
		clientid:    int(clientnum),
		userinfo:    userinfo,
		userinfomap: info,
		name:        info["name"],
		ip:          info["ip"],
		port:        port,
		fov:         fov,
		connecttime: GetUnixTimestamp(),
	}

	LoadPlayerHash(&newplayer)

	log.Printf("[%s/PLAYER] %d|%s|%s\n", srv.Name, clientnum, newplayer.hash, userinfo)

	srv.Players[newplayer.clientid] = newplayer
	return &newplayer
}

func ParseCommand(srv *Server) {
	cmd := ReadByte(&srv.Message)
	switch cmd {
	case PCMDTeleport:
		Teleport(srv)

	case PCMDInvite:
		Invite(srv)
	}
}

package main

import (
	"fmt"
	"log"
	"strconv"
)

/**
 * Loop through all the data from the client
 * and act accordingly
 */
func ParseMessage(srv *Server) {
	msg := &srv.message
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
	v := ReadByte(&srv.message)
	a := ReadByte(&srv.message)

	//victim := findplayer(srv.players, int(v))

	log.Printf("[%s/FRAG] %d > %d\n", srv.name, a, v)
}

/**
 * Received a ping from a client, send a pong to show we're alive
 */
func Pong(srv *Server) {
	if config.Debug > 0 {
		log.Printf("[%s/PING]\n", srv.name)
	}
	WriteByte(SCMDPong, &srv.messageout)
}

/**
 * A print was sent by the server.
 * 1 byte: print level
 * string: the actual message
 */
func ParsePrint(srv *Server) {
	level := ReadByte(&srv.message)
	text := ReadString(&srv.message)
	log.Printf("[%s/PRINT] (%d) %s\n", srv.name, level, text)

	sql := "INSERT INTO logdata (server, msgtype, entry, entrydate) VALUES (?,?,?,?)"
	q, err := db.Query(sql, srv.id, LogTypePrint, text, GetUnixTimestamp())
	if err != nil {
		log.Println(err)
		return
	}
	q.Close()
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

	txt := fmt.Sprintf("[%s/CONNECT] %d|%s|%s", srv.name, p.clientid, info["name"], info["ip"])
	log.Printf("%s\n", txt)
	LogEventToDatabase(srv.id, LogTypeJoin, txt)

	// global
	if isbanned, msg := CheckForBan(&globalbans, p.ip); isbanned == Banned {
		SayPlayer(
			srv,
			p.clientid,
			PRINT_CHAT,
			fmt.Sprintf("Your IP/Userinfo matches a global ban: %s\n", msg),
		)
		KickPlayer(srv, p.clientid)
		return
	}

	// local
	if isbanned, msg := CheckForBan(&srv.bans, p.ip); isbanned == Banned {
		SayPlayer(
			srv,
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
	clientnum := ReadByte(&srv.message)
	srv.players = removeplayer(srv.players, int(clientnum))
	log.Printf("[%s/DISCONNECT] (%d)\n", srv.name, clientnum)
}

/**
 * Server told us what map is currently running. Typically happens
 * when the map changes
 */
func ParseMap(srv *Server) {
	mapname := ReadString(&srv.message)
	srv.currentmap = mapname
	log.Printf("[%s/MAP] %s\n", srv.name, srv.currentmap)
}

func ParsePlayerlist(srv *Server) {
	count := ReadByte(&srv.message)
	log.Printf("[%s/PLAYERLIST] %d\n", srv.name, count)
	for i := 0; i < int(count); i++ {
		_ = ParsePlayer(srv)
	}
}

func ParsePlayer(srv *Server) *Player {
	clientnum := ReadByte(&srv.message)
	userinfo := ReadString(&srv.message)

	if int(clientnum) > srv.maxplayers {
		log.Printf("WARNING: Invalid client number, ignoring\n")
		return nil
	}

	log.Printf("[%s/PLAYER] (%d) %s\n", srv.name, clientnum, userinfo)

	info := UserinfoMap(userinfo)
	port, _ := strconv.Atoi(info["port"])
	fov, _ := strconv.Atoi(info["fov"])
	newplayer := Player{
		clientid: int(clientnum),
		userinfo: userinfo,
		name:     info["name"],
		ip:       info["ip"],
		port:     port,
		fov:      fov,
	}

	// make sure player isn't already in the slice
	for _, p := range srv.players {
		if p.clientid == newplayer.clientid {
			return nil
		}
	}
	srv.players = append(srv.players, newplayer)
	return &newplayer
}

func ParseCommand(srv *Server) {
	cmd := ReadByte(&srv.message)
	switch cmd {
	case PCMDTeleport:
		Teleport(srv)

	case PCMDInvite:
		Invite(srv)
	}
}

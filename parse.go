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
func (cl *Client) ParseMessage() {
	msg := &cl.Message
	for {
		if msg.index >= len(msg.buffer) {
			break
		}

		switch b := ReadByte(msg); b {
		case CMDPing:
			Pong(cl)

		case CMDPrint:
			ParsePrint(cl)

		case CMDMap:
			ParseMap(cl)

		case CMDPlayerList:
			ParsePlayerlist(cl)

		case CMDConnect:
			ParseConnect(cl)

		case CMDDisconnect:
			ParseDisconnect(cl)

		case CMDCommand:
			ParseCommand(cl)

		case CMDFrag:
			ParseFrag(cl)
		}
	}
}

/**
 * A player was fragged.
 * Only two bytes are sent: the clientID of the victim,
 * and of the attacker
 */
func ParseFrag(cl *Client) {
	v := int(ReadByte(&cl.Message))
	a := int(ReadByte(&cl.Message))

	victim := cl.FindPlayer(v)
	attacker := cl.FindPlayer(a)

	if victim == nil {
		return
	}

	log.Printf("[%s/FRAG] %d > %d\n", cl.Name, a, v)

	if attacker == victim || attacker == nil {
		victim.Suicides++
		victim.Frags--
		victim.Deaths++
	} else {
		attacker.Frags++
		victim.Deaths++
	}
}

/**
 * Received a ping from a client, send a pong to show we're alive
 */
func Pong(cl *Client) {
	if q2a.config.Debug > 1 {
		log.Printf("[%s/PING]\n", cl.Name)
	}
	cl.PingCount++
	WriteByte(SCMDPong, &cl.MessageOut)

	// close to once per hour
	if (cl.PingCount & 63) == 0 {
		RotateKeys(cl)
	}
}

/**
 * A print was sent by the server.
 * 1 byte: print level
 * string: the actual message
 */
func ParsePrint(cl *Client) {
	level := ReadByte(&cl.Message)
	text := ReadString(&cl.Message)

	// remove newline
	stripped := text[0 : len(text)-1]

	switch level {
	case PRINT_CHAT:
		cl.SendToWebsiteFeed(stripped, FeedChat)
		LogChat(cl, text)
		log.Printf("[%s/PRINT] (%d) %s\n", cl.Name, level, stripped)
	case PRINT_MEDIUM:
		ParseObituary(text)
	}
}

/**
 * A player connected to the a q2 server
 */
func ParseConnect(cl *Client) {
	p := ParsePlayer(cl)

	if p == nil {
		return
	}

	info := UserinfoMap(p.Userinfo)

	txt := fmt.Sprintf("[%s/CONNECT] %d|%s|%s|%s", cl.Name, p.ClientID, info["name"], info["ip"], p.Hash)
	log.Printf("%s\n", txt)
	LogEventToDatabase(cl.ID, LogTypeJoin, txt)

	wstxt := fmt.Sprintf("[CONNECT] %s [%s]", info["name"], info["ip"])
	cl.SendToWebsiteFeed(wstxt, FeedJoinPart)

	// local rules first
	matched, rules := cl.CheckRules(p, cl.Rules)
	if matched {
		for _, r := range rules {
			switch r.Type {
			case "msg":
				cl.SayPlayer(p.ClientID, PRINT_CHAT, r.Message)
			case "ban":
				KickPlayer(cl, p.ClientID)
			case "mute":
				cl.SayPlayer(p.ClientID, PRINT_CHAT, r.Message)
				//MutePlayer(cl, p.ClientID, 9999)
			}
		}
	}

	// global
	if isbanned, msg := CheckForBan(&globalbans, p.IP); isbanned == Banned {
		cl.SayPlayer(
			p.ClientID,
			PRINT_CHAT,
			fmt.Sprintf("Your IP/Userinfo matches a global ban: %s\n", msg),
		)
		KickPlayer(cl, p.ClientID)
		return
	}

	// local
	/*
		if isbanned, msg := CheckForBan(&cl.Bans, p.IP); isbanned == Banned {
			cl.SayPlayer(
				p.ClientID,
				PRINT_CHAT,
				fmt.Sprintf("Your IP/Userinfo matches a local ban: %s\n", msg),
			)
			KickPlayer(srv, p.ClientID)
		}
	*/
}

/**
 * A player disconnected from a q2 server
 */
func ParseDisconnect(cl *Client) {
	clientnum := int(ReadByte(&cl.Message))

	if clientnum < 0 || clientnum > cl.MaxPlayers {
		log.Printf("Invalid client number: %d\n%s\n", clientnum, hex.Dump(cl.Message.buffer))
		return
	}

	pl := cl.FindPlayer(clientnum)

	wstxt := fmt.Sprintf("[DISCONNECT] %s [%s]", pl.Name, pl.IP)
	cl.SendToWebsiteFeed(wstxt, FeedJoinPart)

	log.Printf("[%s/DISCONNECT] %d|%s\n", cl.Name, clientnum, pl.Name)
	cl.RemovePlayer(clientnum)
}

/**
 * Server told us what map is currently running. Typically happens
 * when the map changes
 */
func ParseMap(cl *Client) {
	mapname := ReadString(&cl.Message)
	cl.CurrentMap = mapname
	log.Printf("[%s/MAP] %s\n", cl.Name, cl.CurrentMap)
}

func ParseObituary(text string) {
	log.Printf("Obit: %s\n", text)
}

func ParsePlayerlist(cl *Client) {
	count := ReadByte(&cl.Message)
	log.Printf("[%s/PLAYERLIST] %d\n", cl.Name, count)
	for i := 0; i < int(count); i++ {
		_ = ParsePlayer(cl)
	}
}

func ParsePlayer(cl *Client) *Player {
	clientnum := ReadByte(&cl.Message)
	userinfo := ReadString(&cl.Message)

	if int(clientnum) > cl.MaxPlayers {
		log.Printf("WARNING: Invalid client number, ignoring\n")
		return nil
	}

	info := UserinfoMap(userinfo)
	port, _ := strconv.Atoi(info["port"])
	fov, _ := strconv.Atoi(info["fov"])
	newplayer := Player{
		ClientID:    int(clientnum),
		Userinfo:    userinfo,
		UserinfoMap: info,
		Name:        info["name"],
		IP:          info["ip"],
		Port:        port,
		FOV:         fov,
		ConnectTime: GetUnixTimestamp(),
	}

	LoadPlayerHash(&newplayer)

	log.Printf("[%s/PLAYER] %d|%s|%s\n", cl.Name, clientnum, newplayer.Hash, userinfo)

	cl.Players[newplayer.ClientID] = newplayer
	cl.PlayerCount++
	return &newplayer
}

func ParseCommand(cl *Client) {
	cmd := ReadByte(&cl.Message)
	switch cmd {
	case PCMDTeleport:
		Teleport(cl)

	case PCMDInvite:
		Invite(cl)
	}
}

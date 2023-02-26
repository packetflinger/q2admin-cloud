package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

// Loop through all the data from the client
// and act accordingly
func (cl *Client) ParseMessage() {
	msg := &cl.Message
	for {
		if msg.index >= len(msg.buffer) {
			break
		}

		switch b := ReadByte(msg); b {
		case CMDPing:
			cl.Pong()

		case CMDPrint:
			cl.ParsePrint()

		case CMDMap:
			cl.ParseMap()

		case CMDPlayerList:
			cl.ParsePlayerlist()

		case CMDPlayerUpdate:
			cl.ParsePlayerUpdate()

		case CMDConnect:
			cl.ParseConnect()

		case CMDDisconnect:
			cl.ParseDisconnect()

		case CMDCommand:
			cl.ParseCommand()

		case CMDFrag:
			cl.ParseFrag()
		}
	}
}

// A player was fragged.
//
// Only two bytes are sent: the clientID of the victim,
// and of the attacker. The means of death are determined
// by parsing the obituary print. For self and environmental
// frags, the attacker and victim will be the same.
func (cl *Client) ParseFrag() {
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

// Received a ping from a client, send a pong to show we're alive
func (cl *Client) Pong() {
	if q2a.config.Debug > 1 {
		log.Printf("[%s/PING]\n", cl.Name)
	}
	cl.PingCount++
	WriteByte(SCMDPong, &cl.MessageOut)

	// once per hour-ish
	if (cl.PingCount & 63) == 0 {
		cl.RotateKeys()
	}
}

// A print was sent by the server.
//
// 1 byte: print level
// string: the actual message
func (cl *Client) ParsePrint() {
	level := ReadByte(&cl.Message)
	text := ReadString(&cl.Message)

	// remove newline
	stripped := text[0 : len(text)-1]

	switch level {
	case PRINT_CHAT:
		cl.SendToWebsiteFeed(stripped, FeedChat)
		cl.LogChat(stripped)
		log.Printf("[%s/PRINT] (%d) %s\n", cl.Name, level, stripped)
	case PRINT_MEDIUM:
		cl.ParseObituary(stripped)
	}
}

// A player connected to the a q2 client.
//
// 1. look up their PTR record
// 2. Parse their userinfo
// 3. Log the connection
// 4. Apply any rules that match them
func (cl *Client) ParseConnect() {
	p := cl.ParsePlayer()

	if p == nil {
		return
	}

	// DNS PTR lookup, take the first one
	ptr, _ := net.LookupAddr(p.IP)
	if len(ptr) > 0 {
		p.Hostname = ptr[0]
	}

	info := p.UserinfoMap

	txt := fmt.Sprintf("[%s/CONNECT] %d|%s|%s|%s", cl.Name, p.ClientID, info["name"], info["ip"], p.UserInfoHash)
	log.Printf("%s\n", txt)

	cl.LogPlayer(p)

	wstxt := fmt.Sprintf("[CONNECT] %s [%s]", info["name"], info["ip"])
	cl.SendToWebsiteFeed(wstxt, FeedJoinPart)

	// add a slight delay when processing rules
	go func() {
		time.Sleep(2 * time.Second)
		cl.ApplyRules(p)
	}()
}

// A player disconnected from a q2 server
func (cl *Client) ParseDisconnect() {
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

// Client told us what map is currently running. Typically happens
// when the map changes
func (cl *Client) ParseMap() {
	mapname := ReadString(&cl.Message)
	cl.CurrentMap = mapname
	log.Printf("[%s/MAP] %s\n", cl.Name, cl.CurrentMap)
}

// An obit for every frag is sent from a client.
//
// Called from ParsePrint()
func (cl *Client) ParseObituary(obit string) {
	death, err := cl.CalculateDeath(obit)
	if err != nil {
		return
	}
	log.Printf(
		"Obituary: %s[%d] -> %s[%d] (%d)\n",
		death.Murderer.Name,
		death.Murderer.ClientID,
		death.Victim.Name,
		death.Victim.ClientID,
		death.Means,
	)
}

// Client sent a playerlist message.
// 1 byte is quantity
// then that number of players are sent
func (cl *Client) ParsePlayerlist() {
	count := ReadByte(&cl.Message)
	log.Printf("[%s/PLAYERLIST] %d\n", cl.Name, count)
	for i := 0; i < int(count); i++ {
		_ = cl.ParsePlayer()
	}
}

// Parse a player message from a client and build a
// player struct here
//
// Called any time a player msg is sent, usually on
// join or new map
func (cl *Client) ParsePlayer() *Player {
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
		ClientID:     int(clientnum),
		Userinfo:     userinfo,
		UserInfoHash: MD5Hash(userinfo),
		UserinfoMap:  info,
		Name:         info["name"],
		IP:           info["ip"],
		Port:         port,
		FOV:          fov,
		ConnectTime:  GetUnixTimestamp(),
		Cookie:       info["cl_cookie"],
		Client:       cl,
	}

	log.Printf("[%s/PLAYER] %d|%s|%s\n", cl.Name, clientnum, newplayer.UserInfoHash, userinfo)

	cl.Players[newplayer.ClientID] = newplayer
	cl.PlayerCount++
	return &newplayer
}

// A command was issued from a player on a client
func (cl *Client) ParseCommand() {
	cmd := ReadByte(&cl.Message)
	switch cmd {
	case PCMDTeleport:
		cl.Teleport()

	case PCMDInvite:
		cl.Invite()
	}
}

// A player changed their userinfo, reparse it and re-apply rules
func (cl *Client) ParsePlayerUpdate() {
	clientnum := ReadByte(&cl.Message)
	userinfo := ReadString(&cl.Message)
	hash := MD5Hash(userinfo)

	player := cl.FindPlayer(int(clientnum))

	// nothing we care about changed
	if hash == player.UserInfoHash {
		return
	}

	info := UserinfoMap(userinfo)
	player.UserinfoMap = info
	player.Name = info["name"]
	player.FOV, _ = strconv.Atoi(info["fov"])
	player.Cookie = info["cl_cookie"]
	player.UserInfoHash = hash

	cl.ApplyRules(player)

	if player.Cookie == "" {
		player.SetupCookie()
	}
}

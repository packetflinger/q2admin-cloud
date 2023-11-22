package server

import (
	"encoding/hex"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/util"
)

// Loop through all the data from the client
// and act accordingly
func ParseMessage(cl *client.Client) {
	msg := &cl.Message
	for {
		if msg.Index >= len(msg.Buffer) {
			break
		}

		switch b := msg.ReadByte(); b {
		case CMDPing:
			Pong(cl)

		case CMDPrint:
			ParsePrint(cl)

		case CMDMap:
			ParseMap(cl)

		case CMDPlayerList:
			ParsePlayerlist(cl)

		case CMDPlayerUpdate:
			ParsePlayerUpdate(cl)

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

// A player was fragged.
//
// Only two bytes are sent: the clientID of the victim,
// and of the attacker. The means of death are determined
// by parsing the obituary print. For self and environmental
// frags, the attacker and victim will be the same.
func ParseFrag(cl *client.Client) {
	msg := &cl.Message
	v := msg.ReadByte()
	a := msg.ReadByte()

	victim := cl.FindPlayer(int(v))
	attacker := cl.FindPlayer(int(a))

	if victim == nil {
		return
	}

	cl.Log.Println("FRAG", a, ">", v)

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
func Pong(cl *client.Client) {
	if Cloud.Config.GetDebugMode() {
		log.Printf("[%s/PING]\n", cl.Name)
	}
	cl.PingCount++
	(&cl.MessageOut).WriteByte(SCMDPong)

	// once per hour-ish
	if (cl.PingCount & 63) == 0 {
		RotateKeys(cl)
	}
}

// A print was sent by the server.
//
// 1 byte: print level
// string: the actual message
func ParsePrint(cl *client.Client) {
	msg := &cl.Message
	level := msg.ReadByte()
	text := msg.ReadString()

	// remove newline
	stripped := text[0 : len(text)-1]

	switch level {
	case PRINT_CHAT:
		//cl.SendToWebsiteFeed(stripped, api.FeedChat)
		//cl.LogChat(stripped)
		cl.Log.Printf("PRINT (%d) %s\n", level, stripped)
	case PRINT_MEDIUM:
		ParseObituary(cl, stripped)
	}
}

// A player connected to the a q2 client.
//
// 1. look up their PTR record
// 2. Parse their userinfo
// 3. Log the connection
// 4. Apply any rules that match them
func ParseConnect(cl *client.Client) {
	p := ParsePlayer(cl)

	if p == nil {
		return
	}

	// DNS PTR lookup, take the first one
	ptr, _ := net.LookupAddr(p.IP)
	if len(ptr) > 0 {
		p.Hostname = ptr[0]
	}

	info := p.UserinfoMap

	cl.Log.Printf("CONNECT %d|%s|%s\n", p.ClientID, info["name"], info["ip"])

	//LogPlayer(cl, p, )

	//wstxt := fmt.Sprintf("[CONNECT] %s [%s]", info["name"], info["ip"])
	//cl.SendToWebsiteFeed(wstxt, api.FeedJoinPart)

	// add a slight delay when processing rules
	go func() {
		time.Sleep(2 * time.Second)
	}()
}

// A player disconnected from a q2 server
func ParseDisconnect(cl *client.Client) {
	clientnum := int((&cl.Message).ReadByte())

	if clientnum < 0 || clientnum > cl.MaxPlayers {
		log.Printf("Invalid client number: %d\n%s\n", clientnum, hex.Dump(cl.Message.Buffer))
		return
	}

	pl := cl.FindPlayer(clientnum)

	//wstxt := fmt.Sprintf("[DISCONNECT] %s [%s]", pl.Name, pl.IP)
	//cl.SendToWebsiteFeed(wstxt, api.FeedJoinPart)

	cl.Log.Printf("DISCONNECT %d|%s\n", clientnum, pl.Name)
	cl.RemovePlayer(clientnum)
}

// Client told us what map is currently running. Typically happens
// when the map changes
func ParseMap(cl *client.Client) {
	mapname := (&cl.Message).ReadString()
	cl.CurrentMap = mapname
	cl.Log.Println("MAP", cl.CurrentMap)
}

// An obit for every frag is sent from a client.
//
// Called from ParsePrint()
func ParseObituary(cl *client.Client, obit string) {
	death, err := cl.CalculateDeath(obit)
	if err != nil {
		return
	}
	cl.Log.Printf(
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
func ParsePlayerlist(cl *client.Client) {
	count := (&cl.Message).ReadByte()
	cl.Log.Println("PLAYERLIST", count)
	for i := 0; i < int(count); i++ {
		_ = ParsePlayer(cl)
	}
}

// Parse a player message from a client and build a
// player struct here
//
// Called any time a player msg is sent, usually on
// join or new map
func ParsePlayer(cl *client.Client) *client.Player {
	msg := &cl.Message
	clientnum := msg.ReadByte()
	userinfo := msg.ReadString()

	if int(clientnum) > cl.MaxPlayers {
		cl.Log.Println("WARN: invalid client number:", clientnum)
		return nil
	}

	info := client.UserinfoMap(userinfo)
	port, _ := strconv.Atoi(info["port"])
	fov, _ := strconv.Atoi(info["fov"])
	newplayer := client.Player{
		ClientID:     int(clientnum),
		Userinfo:     userinfo,
		UserInfoHash: crypto.MD5Hash(userinfo),
		UserinfoMap:  info,
		Name:         info["name"],
		IP:           info["ip"],
		Port:         port,
		FOV:          fov,
		ConnectTime:  util.GetUnixTimestamp(),
		Cookie:       info["cl_cookie"],
		Client:       cl,
	}

	cl.Log.Printf("PLAYER %d|%s|%s\n", clientnum, newplayer.UserInfoHash, userinfo)

	cl.Players[newplayer.ClientID] = newplayer
	cl.PlayerCount++
	return &newplayer
}

// A command was issued from a player on a client
func ParseCommand(cl *client.Client) {
	cmd := (&cl.Message).ReadByte()
	switch cmd {
	case PCMDTeleport:
		Teleport(cl)

	case PCMDInvite:
		Invite(cl)
	}
}

// A player changed their userinfo, reparse it and re-apply rules
func ParsePlayerUpdate(cl *client.Client) {
	msg := &cl.Message
	clientnum := msg.ReadByte()
	userinfo := msg.ReadString()
	hash := crypto.MD5Hash(userinfo)

	player := cl.FindPlayer(int(clientnum))

	// nothing we care about changed
	if hash == player.UserInfoHash {
		return
	}

	info := client.UserinfoMap(userinfo)
	player.UserinfoMap = info
	player.Name = info["name"]
	player.FOV, _ = strconv.Atoi(info["fov"])
	player.Cookie = info["cl_cookie"]
	player.UserInfoHash = hash

	if player.Cookie == "" {
		SetupPlayerCookie(cl, player)
	}
}

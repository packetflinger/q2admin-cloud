package server

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/crypto"
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
	var aName string
	msg := &cl.Message
	fmt.Println(hex.Dump(msg.Buffer))
	v := msg.ReadByte()
	a := msg.ReadByte()

	victim, err := cl.FindPlayer(int(v))
	if err != nil {
		fmt.Println("ParseFrag():", err)
		cl.Log.Println("error in ParseFrag():", err)
		cl.SSHPrintln("error in ParseFrag(): " + err.Error())
		return
	}
	attacker, err := cl.FindPlayer(int(a))
	if err != nil {
		aName = "World/Self"
	} else {
		aName = attacker.Name
	}

	if attacker == victim || attacker == nil {
		victim.Suicides++
		victim.Frags--
	} else {
		attacker.Frags++
	}
	victim.Deaths++
	cl.Log.Println("FRAG", aName, ">", victim.Name)
	cl.SSHPrintln("FRAG " + aName + " > " + victim.Name)
}

// Received a ping from a client, send a pong to show we're alive
func Pong(cl *client.Client) {
	if srv.config.GetDebugMode() {
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
		cl.Log.Println("CHAT", stripped)
		msgColor := ansiCode{foreground: ColorGreen, bold: true}.Render()
		msg := fmt.Sprintf("%s%s%s", msgColor, stripped, AnsiReset)
		cl.SSHPrintln(msg)
	case PRINT_HIGH:
		cl.Log.Println("PRINT", stripped)
		msgColor := ansiCode{foreground: ColorBlack, background: ColorLightGray}.Render()
		cl.SSHPrintln(msgColor + stripped + AnsiReset)
	case PRINT_MEDIUM:
		//ParseObituary(cl, stripped)
		cl.Log.Println("PRINT", stripped)
		msgColor := ansiCode{foreground: ColorDarkGray, background: ColorWhite}.Render()
		cl.SSHPrintln(msgColor + stripped + AnsiReset)
	}

	// re-stifle if needed
	if level == PRINT_CHAT {
		players, err := cl.GetPlayerFromPrint(stripped)
		if err != nil {
			cl.Log.Println(err)
			return
		}
		for _, p := range players {
			if p.Stifled {
				MutePlayer(cl, p, p.StifleLength)
			}
		}
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

	//info := p.UserinfoMap
	msg := fmt.Sprintf("%-20s[%d] %-20q %s", "CONNECT:", p.ClientID, p.Name, p.IP)
	//cl.Log.Printf("CONNECT %d|%s|%s\n", p.ClientID, info["name"], info["ip"])
	cl.Log.Printf(msg)
	cl.SSHPrintln(msg)
	match, rules := CheckRules(p, cl.Rules)

	// add a slight delay when processing rules
	go func() {
		time.Sleep(1 * time.Second)
		if match {
			p.Rules = rules
			ApplyMatchedRules(p, rules)
		}
	}()
}

// A player disconnected from a q2 server
func ParseDisconnect(cl *client.Client) {
	clientnum := int((&cl.Message).ReadByte())

	if clientnum < 0 || clientnum > cl.MaxPlayers {
		log.Printf("Invalid client number: %d\n%s\n", clientnum, hex.Dump(cl.Message.Buffer))
		return
	}

	pl, err := cl.FindPlayer(clientnum)
	if err != nil {
		cl.Log.Println("error in ParseDisconnect():", err)
		return
	}

	//wstxt := fmt.Sprintf("[DISCONNECT] %s [%s]", pl.Name, pl.IP)
	//cl.SendToWebsiteFeed(wstxt, api.FeedJoinPart)
	msg := fmt.Sprintf("%-20s[%d] %-20q %s", "DISCONNECT:", pl.ClientID, pl.Name, pl.IP)
	cl.Log.Printf(msg)
	cl.SSHPrintln(msg)
	cl.RemovePlayer(clientnum)
}

// Client told us what map is currently running. Typically happens
// when the map changes
func ParseMap(cl *client.Client) {
	mapname := (&cl.Message).ReadString()
	cl.CurrentMap = mapname
	msg := fmt.Sprintf("%-20s%q", "MAP_CHANGE:", cl.CurrentMap)
	cl.Log.Println(msg)
	cl.SSHPrintln(msg)
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
		"Obituary: %s[%d] -> %s[%d] (%s)\n",
		death.Murderer.Name,
		death.Murderer.ClientID,
		death.Victim.Name,
		death.Victim.ClientID,
		death.MeansToString(),
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
		ConnectTime:  time.Now().Unix(),
		Cookie:       info["cl_cookie"],
		Client:       cl,
	}

	cl.Log.Printf("PLAYER %d|%s|%s\n", clientnum, newplayer.UserInfoHash, userinfo)

	cl.Players[newplayer.ClientID] = newplayer
	cl.PlayerCount++
	return &cl.Players[newplayer.ClientID]
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

	player, err := cl.FindPlayer(int(clientnum))
	if err != nil {
		cl.Log.Println("error in ParsePlayerUpdate():", err)
		cl.SSHPrintln("error in ParsePlayerUpdate(): " + err.Error())
		return
	}

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

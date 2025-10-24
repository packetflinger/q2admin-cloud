package backend

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/frontend"
)

type greeting struct {
	uuid       string
	version    int
	port       int
	maxPlayers int
	encrypted  bool
	challenge  []byte
}

// Loop through all the data from the frontend and act accordingly
func ParseMessage(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	msg := &fe.Message
	for {
		if msg.Index >= len(msg.Data) {
			break
		}

		switch b := msg.ReadByte(); b {
		case CMDPing:
			Pong(fe)

		case CMDPrint:
			ParsePrint(fe)

		case CMDMap:
			ParseMap(fe)

		case CMDPlayerList:
			ParsePlayerlist(fe)

		case CMDPlayerUpdate:
			ParsePlayerUpdate(fe)

		case CMDConnect:
			ParseConnect(fe)

		case CMDDisconnect:
			ParseDisconnect(fe)

		case CMDCommand:
			ParseCommand(fe)
		}
	}
}

func ParseGreeting(msg *message.Buffer) (greeting, error) {
	if msg == nil {
		return greeting{}, fmt.Errorf("null msg buffer")
	}
	if msg.Length < GreetingLength {
		return greeting{}, fmt.Errorf("short greeting (%d)", msg.Length)
	}
	return greeting{
		uuid:       msg.ReadString(),
		version:    int(msg.ReadLong()),
		port:       int(msg.ReadShort()),
		maxPlayers: int(msg.ReadByte()),
		encrypted:  msg.ReadByte() == 1,
		challenge:  msg.ReadData(crypto.RSAKeyLength),
	}, nil
}

// Parse the client's response to the server's auth challenge and compare the
// results.
func (s *Backend) AuthenticateClient(msg *message.Buffer, fe *frontend.Frontend) (bool, error) {
	if msg == nil {
		return false, fmt.Errorf("null msg buffer")
	}
	if fe == nil {
		return false, fmt.Errorf("null frontend")
	}
	if msg.Length != crypto.RSAKeyLength+3 {
		return false, fmt.Errorf("[%s] invalid frontend auth length (%d)", fe.Name, msg.Length)
	}

	cmd := msg.ReadByte()
	if cmd != CMDAuth {
		return false, fmt.Errorf("[%s] not a frontend auth message", fe.Name)
	}

	cipher := msg.ReadData(msg.ReadShort())
	if len(cipher) == 0 {
		return false, fmt.Errorf("[%s] invalid cipher length", fe.Name)
	}

	digestFromFrontend, err := crypto.PrivateDecrypt(s.privateKey, cipher)
	if err != nil {
		be.Logf(LogLevelNormal, "[%s] private key error: %v", fe.Name, err)
	}

	digestFromServer, err := crypto.MessageDigest(fe.Challenge)
	if err != nil {
		be.Logf(LogLevelInfo, "[%s] hashing error: %v\n", fe.Name, err)
	}

	return bytes.Equal(digestFromFrontend, digestFromServer), nil
}

// A player was fragged.
//
// Only two bytes are sent: the clientID of the victim, and of the attacker.
// The means of death are determined by parsing the obituary print. For self
// and environmental frags, the attacker and victim will be the same.
func ParseFrag(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	var aName string
	msg := &fe.Message
	v := msg.ReadByte()
	a := msg.ReadByte()

	victim, err := fe.FindPlayer(int(v))
	if err != nil {
		fmt.Println("ParseFrag():", err)
		fe.Log.Println("error in ParseFrag():", err)
		fe.SSHPrintln("error in ParseFrag(): " + err.Error())
		return
	}
	attacker, err := fe.FindPlayer(int(a))
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
	fe.Log.Println("FRAG", aName, ">", victim.Name)
	fe.SSHPrintln("FRAG " + aName + " > " + victim.Name)
}

// Received a ping from a client, send a pong to show we're alive
func Pong(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	if be.config.GetVerboseLevel() >= LogLevelDeveloperPlus {
		log.Printf("[%s/PING]\n", fe.Name)
	}
	fe.PingCount++
	(&fe.MessageOut).WriteByte(SCMDPong)

	// once per hour-ish
	if (fe.PingCount & 63) == 0 {
		RotateKeys(fe)
	}
}

// A print was sent by the server.
//
// 1 byte: print level
// string: the actual message
func ParsePrint(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	msg := &fe.Message
	level := msg.ReadByte()
	text := msg.ReadString()

	// remove newline
	stripped := text[0 : len(text)-1]

	switch level {
	case PRINT_CHAT:
		fe.Log.Println("CHAT", stripped)
		msgColor := ansiCode{foreground: ColorGreen, bold: true}.Render()
		msg := fmt.Sprintf("%s%s%s", msgColor, stripped, AnsiReset)
		fe.SSHPrintln(msg)
	case PRINT_HIGH:
		fe.Log.Println("PRINT", stripped)
		msgColor := ansiCode{foreground: ColorBlack, background: ColorLightGray}.Render()
		fe.SSHPrintln(msgColor + stripped + AnsiReset)
	case PRINT_MEDIUM:
		ParseObituary(fe, stripped)
		//cl.Log.Println("PRINT", stripped)
		//msgColor := ansiCode{foreground: ColorDarkGray, background: ColorWhite}.Render()
		//cl.SSHPrintln(msgColor + stripped + AnsiReset)
	}

	// re-stifle if needed
	if level == PRINT_CHAT {
		players, err := fe.GetPlayerFromPrint(stripped)
		if err != nil {
			fe.Log.Println(err)
			return
		}
		for _, p := range players {
			if p.Stifled {
				MutePlayer(fe, p, p.StifleLength)
			}
		}
	}
}

// A player connected to the a frontend.
//
// - look up their PTR record
// - Log the connection
// - Apply any rules that match them
func ParseConnect(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	p := ParsePlayer(fe)
	if p == nil {
		return
	}

	// DNS resolution can take time (up to seconds) to get a response, so
	// logging a connect should be done concurrently to prevent blocking. Rules
	// can depend on DNS names (*.isp.com) so we need to wait for a PTR before
	// processing rules.
	go func() {
		ptr, err := net.LookupAddr(p.IP)
		if err != nil {
			log.Printf("error looking up dns for %s[%s]: %v\n", p.Name, p.IP, err)
		}
		if len(ptr) > 0 {
			p.Hostname = ptr[0] // just take the first address
		}

		msg := fmt.Sprintf("%-20s[%d] %-20q %s", "CONNECT:", p.ClientID, p.Name, p.IP)
		fe.Log.Printf("%s", msg)
		fe.SSHPrintln(msg)

		// add a slight delay when processing rules
		time.Sleep(1 * time.Second)

		match, rules := CheckRules(p, append(fe.Rules, be.rules...))
		if match {
			p.Rules = rules
			ApplyMatchedRules(p, rules)
		}
	}()
}

// A player disconnected from a q2 server
func ParseDisconnect(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	clientnum := int((&fe.Message).ReadByte())

	if clientnum < 0 || clientnum > fe.MaxPlayers {
		log.Printf("Invalid client number: %d\n%s\n", clientnum, hex.Dump(fe.Message.Data))
		return
	}

	pl, err := fe.FindPlayer(clientnum)
	if err != nil {
		fe.Log.Println("error in ParseDisconnect():", err)
		return
	}

	msg := fmt.Sprintf("%-20s[%d] %-20q %s", "DISCONNECT:", pl.ClientID, pl.Name, pl.IP)
	fe.Log.Printf("%s", msg)
	fe.SSHPrintln(msg)
	fe.RemovePlayer(clientnum)
}

// Frontend told us what map is currently running. Typically happens when the
// map changes.
func ParseMap(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	mapname := (&fe.Message).ReadString()
	fe.PreviousMap = fe.CurrentMap
	fe.CurrentMap = mapname
	msg := fmt.Sprintf("%-20s %q (was %q)", "MAP_CHANGE:", fe.CurrentMap, fe.PreviousMap)
	fe.Log.Println(msg)
	fe.SSHPrintln(msg)
}

// An obit for every frag is sent from a client.
//
// Called from ParsePrint()
func ParseObituary(fe *frontend.Frontend, obit string) {
	if fe == nil || obit == "" {
		return
	}
	death, err := fe.CalculateDeath(obit)
	if err != nil {
		return
	}
	if death.Victim == nil {
		return
	}
	var logObit string
	// single-sided frag
	if death.Murderer == nil {
		logObit = fmt.Sprintf("DEATH: %s[%d] (%s)",
			death.Victim.Name,
			death.Victim.ClientID,
			death.MeansToString(),
		)
	} else {
		logObit = fmt.Sprintf("DEATH: %s[%d] -> %s[%d] (%s)",
			death.Murderer.Name,
			death.Murderer.ClientID,
			death.Victim.Name,
			death.Victim.ClientID,
			death.MeansToString(),
		)
	}
	fe.Log.Printf("%s", logObit)
	fe.SSHPrintln(logObit)
}

// Client sent a playerlist message.
// 1 byte is quantity
// then that number of players are sent
func ParsePlayerlist(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	count := (&fe.Message).ReadByte()
	fe.Log.Println("PLAYERLIST", count)
	for i := 0; i < int(count); i++ {
		_ = ParsePlayer(fe)
	}
}

// Parse a player message from a client and build a player struct here. Called
// any time a player msg is sent, usually on join or new map.
func ParsePlayer(fe *frontend.Frontend) *frontend.Player {
	if fe == nil {
		return nil
	}
	msg := &fe.Message
	clientnum := msg.ReadByte()
	userinfo := msg.ReadString()
	clientVersion := msg.ReadString()

	if int(clientnum) > fe.MaxPlayers {
		fe.Log.Println("WARN: invalid client number:", clientnum)
		return nil
	}

	info := frontend.UserinfoMap(userinfo)
	port, err := strconv.Atoi(info["port"])
	if err != nil {
		port = 0
	}
	fov, err := strconv.Atoi(info["fov"])
	if err != nil {
		fov = 0
	}
	newplayer := frontend.Player{
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
		Frontend:     fe,
		Version:      clientVersion,
	}

	fe.Log.Printf("PLAYER %d|%s|%s\n", clientnum, newplayer.UserInfoHash, userinfo)

	fe.Players[newplayer.ClientID] = newplayer
	fe.PlayerCount++

	err = fe.AddPlayer(&newplayer)
	if err != nil {
		log.Println(err)
	}
	return &fe.Players[newplayer.ClientID]
}

// A command was issued from a player on a client
func ParseCommand(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	cmd := (&fe.Message).ReadByte()
	switch cmd {
	case PCMDTeleport:
		Teleport(fe)

	case PCMDInvite:
		Invite(fe)
	}
}

// A player changed their userinfo, reparse it and re-apply rules
func ParsePlayerUpdate(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	msg := &fe.Message
	clientnum := msg.ReadByte()
	userinfo := msg.ReadString()
	hash := crypto.MD5Hash(userinfo)

	player, err := fe.FindPlayer(int(clientnum))
	if err != nil {
		// sometimes player updates are sent before the actual player join
		// message because the join message is waiting on things like the
		// client version and VPN checks. It's a (usually losing) race
		// condition, just swallow the error.
		return
	}

	// nothing we care about changed
	if hash == player.UserInfoHash {
		return
	}

	info := frontend.UserinfoMap(userinfo)
	player.UserinfoMap = info
	player.Name = info["name"]
	player.FOV, _ = strconv.Atoi(info["fov"])
	player.Cookie = info["cl_cookie"]
	player.UserInfoHash = hash

	if player.Cookie == "" {
		SetupPlayerCookie(fe, player)
	}
}

package main

import (
	//"bufio"
	"database/sql"
	"flag"
	"os/signal"
	"strings"

	//"encoding/hex"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

var (
	Configfile = flag.String("c", "q2a.json", "The main config file")
	Servers    = []Server{}
)

const (
	ProtocolMagic   = 1128346193 // "Q2AC"
	versionRequired = 300        // git revision number
	challengeLength = 16         // bytes
	AESBlockLength  = 16         // 128 bit
	AESIVLength     = 16         // 128 bit
	SessionName     = "q2asess"  // website cookie name
	TeleportWidth   = 80         // max chars per line for teleport replies
)

/**
 * Global variables
 */
var config Config        // the local config
var q2a AdminServer      // this server
var db *sql.DB           // our database connection (sqlite3)
var servers = []Server{} // the slice of game servers we manage

/**
 * Commands sent from the Q2 server to us
 */
const (
	_             = iota
	CMDHello      // server connect
	CMDQuit       // server disconnect
	CMDConnect    // player connect
	CMDDisconnect // player disconnect
	CMDPlayerList
	CMDPlayerUpdate
	CMDPrint
	CMDCommand
	CMDPlayers
	CMDFrag
	CMDMap
	CMDPing
	CMDAuth
)

/**
 * Commands we send back to the Q2 server
 */
const (
	_ = iota
	SCMDHelloAck
	SCMDError
	SCMDPong
	SCMDCommand
	SCMDSayClient
	SCMDSayAll
	SCMDAuth
	SCMDTrusted
	SCMDKey
	SCMDGetPlayers
)

/**
 * Player commands, players can issue this from their client
 */
const (
	PCMDTeleport = iota
	PCMDInvite
	PCMDWhois
	PCMDReport
)

/**
 * Print levels
 */
const (
	PRINT_LOW    = iota // pickups
	PRINT_MEDIUM        // obituaries (white/grey, no sound)
	PRINT_HIGH          // important stuff
	PRINT_CHAT          // highlighted, sound
)

/**
 * Log types, used in the database
 */
const (
	LogTypePrint = iota
	LogTypeJoin
	LogTypePart
	LogTypeConnect
	LogTypeDisconnect
	LogTypeCommand
)

/**
 * Initialize a message buffer
 */
func clearmsg(msg *MessageBuffer) {
	msg.buffer = nil
	msg.index = 0
	msg.length = 0
}

/**
 * Locate the struct of the server for a particular
 * ID, get a pointer to it
 */
func findserver(lookup string) (*Server, error) {
	for i, srv := range servers {
		if srv.UUID == lookup {
			return &servers[i], nil
		}
	}

	return nil, errors.New("unknown server")
}

/**
 * Send all messages in the outgoing queue to the gameserver
 */
func (srv *Server) SendMessages() {
	if !srv.Connected {
		return
	}

	// keys have been exchanged, encrypt the message
	if srv.Trusted && srv.Encrypted {
		cipher := SymmetricEncrypt(
			srv.AESKey,
			srv.AESIV,
			srv.MessageOut.buffer[:srv.MessageOut.length])

		clearmsg(&srv.MessageOut)
		srv.MessageOut.buffer = cipher
		srv.MessageOut.length = len(cipher)
	}

	if srv.MessageOut.length > 0 {
		(*srv.Connection).Write(srv.MessageOut.buffer)
		clearmsg(&srv.MessageOut)
	}
}

/**
 * Dates are stored in the database as unix timestamps
 */
func GetUnixTimestamp() int64 {
	return time.Now().Unix()
}

//
// Get current time in HH:MM:SS format
//
func GetTimeNow() string {
	now := time.Now()
	return fmt.Sprintf("%02d:%02d:%02d", now.Hour(), now.Minute(), now.Second())
}

/**
 * Get a time "object" from a database timestamp
 */
func GetTimeFromTimestamp(ts int64) time.Time {
	return time.Unix(ts, 0)
}

func (s *Server) ValidClientID(id int) bool {
	return id >= 0 && id < s.MaxPlayers
}

//
// Each server keeps track of the websocket for people "looking at it".
// When they close the browser or logout, remove the pointer
// to that socket
func (srv *Server) DeleteWebSocket(sock *websocket.Conn) {
	location := -1
	// find it's index first
	for i := range srv.WebSockets {
		if srv.WebSockets[i] == sock {
			location = i
			break
		}
	}

	// wasn't found, forget it
	if location == -1 {
		return
	}

	tempws := srv.WebSockets[0:location]
	tempws = append(tempws, srv.WebSockets[location+1:]...)
	srv.WebSockets = tempws
}

//
// Send the txt string to all the websockets listening
//
func (srv *Server) SendToWebsiteFeed(txt string, decoration int) {
	now := GetTimeNow()

	colored := ""
	switch decoration {
	case FeedChat:
		colored = now + " \\\\e[32m" + txt + "\\\\e[0m"
	case FeedJoinPart:
		colored = now + " \\\\e[33m\\\\e[42m" + txt + "\\\\e[0m"
	default:
		colored = now + " " + txt
	}

	sockets := srv.WebSockets
	for i := range sockets {
		err := sockets[i].WriteMessage(1, []byte(colored))
		if err != nil {
			log.Println(err)
			srv.DeleteWebSocket(srv.WebSockets[i])
		}
	}
}

/**
 * Setup the connection
 * The first message sent should identify the game server
 * and trigger the authentication process
 */
func handleConnection(c net.Conn) {
	log.Printf("Serving %s\n", c.RemoteAddr().String())

	input := make([]byte, 5000)
	var msg MessageBuffer

	_, _ = c.Read(input)
	msg.buffer = input

	magic := ReadLong(&msg)
	if magic != ProtocolMagic {
		// not a valid client, just close connection
		log.Println("Bad magic value in new connection, not a valid client")
		c.Close()
		return
	}

	_ = ReadByte(&msg) // should be CMDHello
	uuid := ReadString(&msg)
	ver := ReadLong(&msg)
	port := ReadShort(&msg)
	maxplayers := ReadByte(&msg)
	enc := ReadByte(&msg)
	clNonce := ReadData(&msg, challengeLength)

	if ver < versionRequired {
		log.Println("Version too old")
		c.Close()
		return
	}

	server, err := findserver(uuid)
	if err != nil {
		// write an error, close socket, returns
		log.Println(err)
		c.Close()
		return
	}
	log.Printf("[%s] connecting...\n", server.Name)

	server.Port = int(port)
	server.Encrypted = int(enc) == 1 // stupid bool conversion
	server.Connection = &c
	server.Connected = true
	server.Version = int(ver)
	server.MaxPlayers = int(maxplayers)
	keyname := fmt.Sprintf("keys/%s.pem", uuid)

	log.Printf("[%s] Loading public key: %s\n", server.Name, keyname)
	pubkey, err := LoadPublicKey(keyname)
	if err != nil {
		log.Printf("Public key error: %s\n", err.Error())
		c.Close()
		return
	}
	server.PublicKey = pubkey

	challengeCipher := Sign(q2a.privatekey, clNonce)
	WriteByte(SCMDHelloAck, &server.MessageOut)
	WriteShort(len(challengeCipher), &server.MessageOut)
	WriteData(challengeCipher, &server.MessageOut)

	/**
	 * if client requests encrypted transit, encrypt the session key/iv
	 * with the client's public key to keep it confidential
	 */
	if server.Encrypted {
		server.AESKey = RandomBytes(AESBlockLength)
		server.AESIV = RandomBytes(AESIVLength)
		blob := append(server.AESKey, server.AESIV...)
		aescipher := PublicEncrypt(server.PublicKey, blob)
		WriteData(aescipher, &server.MessageOut)
	}

	svchallenge := RandomBytes(challengeLength)
	WriteData(svchallenge, &server.MessageOut)

	server.SendMessages()

	// read the client signature
	size, _ := c.Read(input)
	msg.buffer = input
	msg.index = 0
	msg.length = size

	op := ReadByte(&msg) // should be CMDAuth (0x0d)
	if op != CMDAuth {
		c.Close()
		return
	}

	sigsize := ReadShort(&msg)
	clientSignature := ReadData(&msg, int(sigsize))
	verified := VerifySignature(server.PublicKey, svchallenge, clientSignature)

	if verified {
		log.Printf("[%s] signature verified, server trusted\n", server.Name)
	} else {
		log.Printf("[%s] signature verifcation failed...", server.Name)
		c.Close()
		return
	}

	LoadBans(server)
	WriteByte(SCMDTrusted, &server.MessageOut)
	server.SendMessages()
	server.Trusted = true

	server.Players = make([]Player, server.MaxPlayers)

	for {
		input := make([]byte, 5000)
		size, err := c.Read(input)
		if err != nil {
			log.Printf(
				"%s disconnected: %s\n",
				c.RemoteAddr().String(),
				err.Error())
			break
		}

		// decrypt if necessary
		if server.Encrypted && server.Trusted {
			input, size = SymmetricDecrypt(server.AESKey, server.AESIV, input[:size])
		}

		server.Message.buffer = input
		server.Message.index = 0
		server.Message.length = size

		server.ParseMessage()
		server.SendMessages()
	}

	server.Connected = false
	server.Trusted = false
	c.Close()
}

/**
 * Gracefully shutdown everything
 */
func Shutdown() {
	log.Println("Shutting down...")
	db.Close() // not sure if this is necessary
}

/**
 * Entry point
 */
func main() {
	// catch stuff like ctrl+c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		Shutdown()
		os.Exit(1)
	}()

	port := fmt.Sprintf("%s:%d", config.Address, config.Port)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		fmt.Println(err)
		return
	}

	defer listener.Close()

	log.Printf("Listening for gameservers on %s\n", port)

	if config.APIEnabled > 0 {
		go RunHTTPServer()
	}

	for {
		c, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go handleConnection(c)
	}
}

// The file should be just a list of server names one per line
// comments (// and #) and blank lines are allowed
// indenting doesn't matter
func (c Config) ReadServerFile() []string {
	contents, err := os.ReadFile(c.ServersFile)
	if err != nil {
		log.Println(err)
		os.Exit(0)
	}

	srvs := []string{}
	lines := strings.Split(string(contents), "\n")
	for i := range lines {
		trimmed := strings.Trim(lines[i], " \t")
		// remove empty lines
		if trimmed == "" {
			continue
		}
		// remove comments
		if trimmed[0] == '#' || trimmed[0:2] == "//" {
			continue
		}
		srvs = append(srvs, trimmed)
	}
	return srvs
}

/**
 * pre-entry point
 */
func init() {
	flag.Parse()

	log.Println("Loading config:", *Configfile)
	confjson, err := os.ReadFile(*Configfile)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(confjson, &config)
	if err != nil {
		log.Fatal(err)
	}

	rand.Seed(time.Now().Unix())

	log.Println("Loading private key:", config.PrivateKey)
	privkey, err := LoadPrivateKey(config.PrivateKey)
	if err != nil {
		log.Fatalf("Problems loading private key: %s\n", err.Error())
	}

	pubkey := privkey.Public().(*rsa.PublicKey)
	q2a.privatekey = privkey
	q2a.publickey = pubkey

	//LoadGlobalBans()

	db = DatabaseConnect()

	log.Println("Loading servers from:", config.ServersFile)
	//servers = LoadServers(db)
	serverlist := config.ReadServerFile()
	for _, s := range serverlist {
		fmt.Println(s)
		//sv := Server{}
		//sv.ReadDiskFormat(s)
	}

	/* TESTS
	c1 := ServerControls{
		Description: "Ban players using Name1 and Name2 from IP subnet for a month",
		Type:        "ban",
		Address:     "192.168.3.0/24",
		Password:    "",
		Name: []string{
			"Name1",
			"Name2",
		},
		Client:      []string{},
		UserInfoKey: []string{},
		UserinfoVal: []string{},
		Created:     0,
		Length:      86400 * 30,
	}
	c2 := ServerControls{
		Description: "Mute all players using Name3 with specific client, permanently",
		Type:        "mute",
		Address:     "0.0.0.0/0",
		Password:    "",
		Name: []string{
			"Name3",
		},
		Client: []string{
			"q2pro r1504~924ff39 Dec  3 2014 Win32 x86",
		},
		UserInfoKey: []string{},
		UserinfoVal: []string{},
		Created:     0,
		Length:      0,
	}
	c3 := ServerControls{
		Description: "Ban all when using name 'claire' unless valid password, permanently",
		Type:        "ban",
		Address:     "0.0.0.0/0",
		Password:    "meatpopcicle",
		Name: []string{
			"claire",
		},
		Client:      []string{},
		UserInfoKey: []string{},
		UserinfoVal: []string{},
		Created:     0,
		Length:      0,
	}
	c4 := ServerControls{
		Description: "Print msg to client at 10.2.2.2 on connect for a week",
		Type:        "msg",
		Address:     "10.2.2.2/32",
		Message:     "Stop being such an asshole or you'll be muted. Only warning.",
		Password:    "",
		Name:        []string{},
		Client:      []string{},
		UserInfoKey: []string{},
		UserinfoVal: []string{},
		Created:     0,
		Length:      86400 * 7,
	}

	s := Server{
		Name:        "example",
		UUID:        "bcec70f2-2215-48d9-9499-3b817b9207d6",
		Owner:       "joe@joereid.com",
		Description: "Duels and Team Deathmatch in US East",
		IPAddress:   "10.2.2.2",
		Port:        27910,
		Verified:    true,
		Controls: []ServerControls{
			c1,
			c2,
			c3,
			c4,
		},
	}
	s.WriteDiskFormat()
	*/

	/*
		s2 := Server{}
		s2.ReadDiskFormat("example")
		fmt.Println()
		fmt.Println(s2)
	*/

	//for _, s := range servers {
	//	log.Printf("  %-15s %-21s [%s]", s.Name, fmt.Sprintf("%s:%d", s.IPAddress, s.Port), s.UUID)
	//}

	os.Exit(0)
}

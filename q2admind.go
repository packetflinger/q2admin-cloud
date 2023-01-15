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
	Clients    = []Client{}
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
var config Config   // the local config
var q2a AdminServer // this server
var db *sql.DB      // our database connection (sqlite3)

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
func FindClient(lookup string) (*Client, error) {
	for i, cl := range Clients {
		if cl.UUID == lookup {
			return &Clients[i], nil
		}
	}

	return nil, errors.New("unknown server")
}

/**
 * Send all messages in the outgoing queue to the gameserver
 */
func (cl *Client) SendMessages() {
	if !cl.Connected {
		return
	}

	// keys have been exchanged, encrypt the message
	if cl.Trusted && cl.Encrypted {
		cipher := SymmetricEncrypt(
			cl.AESKey,
			cl.AESIV,
			cl.MessageOut.buffer[:cl.MessageOut.length])

		clearmsg(&cl.MessageOut)
		cl.MessageOut.buffer = cipher
		cl.MessageOut.length = len(cipher)
	}

	if cl.MessageOut.length > 0 {
		(*cl.Connection).Write(cl.MessageOut.buffer)
		clearmsg(&cl.MessageOut)
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

func (cl *Client) ValidClientID(id int) bool {
	return id >= 0 && id < cl.MaxPlayers
}

//
// Each server keeps track of the websocket for people "looking at it".
// When they close the browser or logout, remove the pointer
// to that socket
func (cl *Client) DeleteWebSocket(sock *websocket.Conn) {
	location := -1
	// find it's index first
	for i := range cl.WebSockets {
		if cl.WebSockets[i] == sock {
			location = i
			break
		}
	}

	// wasn't found, forget it
	if location == -1 {
		return
	}

	tempws := cl.WebSockets[0:location]
	tempws = append(tempws, cl.WebSockets[location+1:]...)
	cl.WebSockets = tempws
}

//
// Send the txt string to all the websockets listening
//
func (cl *Client) SendToWebsiteFeed(txt string, decoration int) {
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

	sockets := cl.WebSockets
	for i := range sockets {
		err := sockets[i].WriteMessage(1, []byte(colored))
		if err != nil {
			log.Println(err)
			cl.DeleteWebSocket(cl.WebSockets[i])
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

	cl, err := FindClient(uuid)
	if err != nil {
		// write an error, close socket, returns
		log.Println(err)
		c.Close()
		return
	}
	log.Printf("[%s] connecting...\n", cl.Name)

	cl.Port = int(port)
	cl.Encrypted = int(enc) == 1 // stupid bool conversion
	cl.Connection = &c
	cl.Connected = true
	cl.Version = int(ver)
	cl.MaxPlayers = int(maxplayers)
	keyname := fmt.Sprintf("keys/%s.pem", uuid)

	log.Printf("[%s] Loading public key: %s\n", cl.Name, keyname)
	pubkey, err := LoadPublicKey(keyname)
	if err != nil {
		log.Printf("Public key error: %s\n", err.Error())
		c.Close()
		return
	}
	cl.PublicKey = pubkey

	challengeCipher := Sign(q2a.privatekey, clNonce)
	WriteByte(SCMDHelloAck, &cl.MessageOut)
	WriteShort(len(challengeCipher), &cl.MessageOut)
	WriteData(challengeCipher, &cl.MessageOut)

	/**
	 * if client requests encrypted transit, encrypt the session key/iv
	 * with the client's public key to keep it confidential
	 */
	if cl.Encrypted {
		cl.AESKey = RandomBytes(AESBlockLength)
		cl.AESIV = RandomBytes(AESIVLength)
		blob := append(cl.AESKey, cl.AESIV...)
		aescipher := PublicEncrypt(cl.PublicKey, blob)
		WriteData(aescipher, &cl.MessageOut)
	}

	svchallenge := RandomBytes(challengeLength)
	WriteData(svchallenge, &cl.MessageOut)

	cl.SendMessages()

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
	verified := VerifySignature(cl.PublicKey, svchallenge, clientSignature)

	if verified {
		log.Printf("[%s] signature verified, server trusted\n", cl.Name)
	} else {
		log.Printf("[%s] signature verifcation failed...", cl.Name)
		c.Close()
		return
	}

	WriteByte(SCMDTrusted, &cl.MessageOut)
	cl.SendMessages()
	cl.Trusted = true

	cl.Players = make([]Player, cl.MaxPlayers)

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
		if cl.Encrypted && cl.Trusted {
			input, size = SymmetricDecrypt(cl.AESKey, cl.AESIV, input[:size])
		}

		cl.Message.buffer = input
		cl.Message.index = 0
		cl.Message.length = size

		cl.ParseMessage()
		cl.SendMessages()
	}

	cl.Connected = false
	cl.Trusted = false
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

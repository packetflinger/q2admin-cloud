package main

import (
	"database/sql"
	"flag"
	"os/signal"
	"strings"

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
	q2a        RemoteAdminServer // this server
	db         *sql.DB
)

const (
	ProtocolMagic   = 1128346193 // "Q2AC"
	versionRequired = 342        // git revision number
	challengeLength = 16         // bytes
	AESBlockLength  = 16         // 128 bit
	AESIVLength     = 16         // 128 bit
	SessionName     = "q2asess"  // website cookie name
	TeleportWidth   = 80         // max chars per line for teleport replies
)

// Commands sent from the Q2 server to us
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

// Commands we send back to the Q2 server
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

// Player commands, players can issue this from their client
const (
	PCMDTeleport = iota
	PCMDInvite
	PCMDWhois
	PCMDReport
)

// Print levels
const (
	PRINT_LOW    = iota // pickups
	PRINT_MEDIUM        // obituaries (white/grey, no sound)
	PRINT_HIGH          // important stuff
	PRINT_CHAT          // highlighted, sound
)

// Log types, used in the database
const (
	LogTypePrint = iota
	LogTypeJoin
	LogTypePart
	LogTypeConnect
	LogTypeDisconnect
	LogTypeCommand
)

// Initialize a message buffer
func clearmsg(msg *MessageBuffer) {
	msg.buffer = nil
	msg.index = 0
	msg.length = 0
}

// Locate the struct of the server for a particular
// ID, get a pointer to it
func FindClient(lookup string) (*Client, error) {
	for i, cl := range q2a.clients {
		if cl.UUID == lookup {
			return &q2a.clients[i], nil
		}
	}

	return nil, errors.New("unknown server")
}

// Send all messages in the outgoing queue to the client (gameserver)
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

	// only send if there is something to send
	if cl.MessageOut.length > 0 {
		(*cl.Connection).Write(cl.MessageOut.buffer)
		clearmsg(&cl.MessageOut)
	}
}

// Dates are stored in the database as unix timestamps
func GetUnixTimestamp() int64 {
	return time.Now().Unix()
}

// Get current time in HH:MM:SS format
func GetTimeNow() string {
	now := time.Now()
	return fmt.Sprintf("%02d:%02d:%02d", now.Hour(), now.Minute(), now.Second())
}

// Convert unix timestamp to a time struct
func GetTimeFromTimestamp(ts int64) time.Time {
	return time.Unix(ts, 0)
}

// Client number is between 0 and maxplayers
func (cl *Client) ValidClientID(id int) bool {
	return id >= 0 && id < cl.MaxPlayers
}

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

// Setup the connection
// The first message sent should identify the game server
// and trigger the authentication process. Connection
// persists in a goroutine from this function.
//
// Called from main loop when a new connection is made
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

	// If client requests encrypted transit, encrypt the session key/iv
	// with the client's public key to keep it confidential
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

	// main connection loop
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

// Gracefully shut everything down
func Shutdown() {
	log.Println("Shutting down...")
	db.Close() // not sure if this is necessary
}

// start here
func main() {
	initialize()
	// catch stuff like ctrl+c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		Shutdown()
		os.Exit(1)
	}()

	port := fmt.Sprintf("%s:%d", q2a.config.Address, q2a.config.Port)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		fmt.Println(err)
		return
	}

	defer listener.Close()

	log.Printf("Listening for gameservers on %s\n", port)

	if q2a.config.APIEnabled > 0 {
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
//
// Called from initialize() at startup
func (c Config) ReadClientFile() []string {
	contents, err := os.ReadFile(c.ClientsFile)
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

// Read all client names from disk, load their diskformats
// into memory. Add each to
//
// Called from initialize() at startup
func (q2a *RemoteAdminServer) LoadClients() {
	clientlist := q2a.config.ReadClientFile()
	cls := []Client{}
	for _, c := range clientlist {
		cl := Client{}
		err := cl.ReadDiskFormat(c)
		if err != nil {
			continue
		}
		cls = append(cls, cl)
	}
	q2a.clients = cls
}

// This is a renamed "init()" function. Having it named init
// was messing up the unit tests.
//
// Called from main() at startup
func initialize() {
	flag.Parse()

	log.Println("Loading config:", *Configfile)
	confjson, err := os.ReadFile(*Configfile)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(confjson, &q2a.config)
	if err != nil {
		log.Fatal(err)
	}

	rand.Seed(time.Now().Unix())

	log.Println("Loading private key:", q2a.config.PrivateKey)
	privkey, err := LoadPrivateKey(q2a.config.PrivateKey)
	if err != nil {
		log.Fatalf("Problems loading private key: %s\n", err.Error())
	}

	pubkey := privkey.Public().(*rsa.PublicKey)
	q2a.privatekey = privkey
	q2a.publickey = pubkey

	db = DatabaseConnect()

	log.Println("Loading global rules...")
	q2a.ReadGlobalRules()

	log.Println("Loading clients from:", q2a.config.ClientsFile)
	q2a.LoadClients()

	// Read users
	log.Println("Loading users from:", q2a.config.UsersFile)
	users, err := ReadUsersFromDisk(q2a.config.UsersFile)
	if err != nil {
		log.Println(err)
	} else {
		q2a.Users = users
	}

	// Read permissions
	log.Println("Loading user access from:", q2a.config.AccessFile)
	useraccess, err := ReadAccessFromDisk(q2a.config.AccessFile)
	if err != nil {
		log.Println(err)
	} else {
		q2a.access = useraccess
	}

	for _, c := range q2a.clients {
		log.Printf("server: %-25s [%s:%d]", c.Name, c.IPAddress, c.Port)
	}
}

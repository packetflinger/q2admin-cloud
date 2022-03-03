package main

import (
	//"bufio"
	"database/sql"
	"os/signal"

	//"encoding/hex"
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

const (
	versionRequired = 300       // git revision number
	challengeLength = 16        // bytes
	AESBlockLength  = 16        // 128bit
	AESIVLength     = 12        // 96bit
	SessionName     = "q2asess" // website cookie name
	TeleportWidth   = 80        // max chars per line for teleport replies
)

// use a custom buffer struct to keep track of where
// we are in the stream of bytes internally
type MessageBuffer struct {
	buffer []byte
	index  int
	length int // maybe not needed
}

type Player struct {
	clientid         int
	name             string
	userinfo         string
	frags            int
	deaths           int
	suicides         int
	teleports        int
	lastteleport     int64 // actually going
	lastteleportlist int64 // viewing the big list of destinations
	invites          int
	lastinvite       int64
	invitesavailable int
	ip               string
	port             int
	fov              int
}

// this is a Quake 2 Gameserver, and also a client to us
type Server struct {
	id         int // this is the database index
	uuid       string
	owner      int
	index      int
	version    int // what version are we running
	name       string
	ipaddress  string
	port       int // default 27910
	connected  bool
	currentmap string
	enabled    bool
	connection *net.Conn
	players    []Player
	maxplayers int
	message    MessageBuffer
	messageout MessageBuffer
	encrypted  bool
	havekeys   bool
	trusted    bool // signature challenge verified
	publickey  *rsa.PublicKey
	aeskey     []byte // 16 (128bit)
	aesiv      []byte
	bans       []Ban
}

// "this" admin server
type AdminServer struct {
	privatekey *rsa.PrivateKey
	publickey  *rsa.PublicKey
}

// structure of the config file
type Config struct {
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Database   string `json:"database"`
	DBString   string `json:"dbstring"`
	PrivateKey string `json:"privatekey"`
	APIPort    int    `json:"apiport"`
	Debug      int    `json:"debug"`
	APIEnabled int    `json:"enableapi"`
}

var config Config
var q2a AdminServer
var db *sql.DB

/**
 * Commands sent from the Q2 server to us
 */
const (
	_ = iota
	CMDHello
	CMDQuit
	CMDConnect
	CMDDisconnect
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
)

const (
	PCMDTeleport = iota
	PCMDInvite
	PCMDWhois
	PCMDReport
)

const (
	PRINT_LOW    = iota // pickups
	PRINT_MEDIUM        // obituaries (white/grey, no sound)
	PRINT_HIGH          // important stuff
	PRINT_CHAT          // highlighted, sound
)

const (
	LogTypePrint = iota
	LogTypeJoin
	LogTypePart
	LogTypeConnect
	LogTypeDisconnect
	LogTypeCommand
)

/*
var servers = []Server {
    {id: 1, key:1234, name: "dm", ipaddress: "107.174.230.210", port: 27910, enabled: true},
    {id: 2, key:2345, name: "dmx", ipaddress: "107.174.230.210", port: 27911, enabled: true},
    {id: 3, key:4567, name: "tourney", ipaddress: "107.174.230.210", port: 27912, enabled: true},
    {id: 4, key:5678, name: "tourney2", ipaddress: "107.174.230.210", port: 27913, enabled: true},
}
*/

var servers = []Server{}

func clearmsg(msg *MessageBuffer) {
	msg.buffer = nil
	msg.index = 0
	msg.length = 0
}

func findplayer(players []Player, cl int) *Player {
	for i, p := range players {
		if p.clientid == cl {
			return &players[i]
		}
	}

	return nil
}

func removeplayer(players []Player, cl int) []Player {
	var index int
	for i, pl := range players {
		if pl.clientid == cl {
			index = i
			break
		}
	}

	return append(players[:index], players[index+1:]...)
}

func SayPlayer(srv *Server, client int, level int, text string) {
	WriteByte(SCMDSayClient, &srv.messageout)
	WriteByte(byte(client), &srv.messageout)
	WriteByte(byte(level), &srv.messageout)
	WriteString(text, &srv.messageout)
}

func SayEveryone(srv *Server, level int, text string) {
	WriteByte(SCMDSayAll, &srv.messageout)
	WriteByte(byte(level), &srv.messageout)
	WriteString(text, &srv.messageout)
}

/**
 * Take a back-slash delimited string of userinfo and return
 * a key/value map
 */
func UserinfoMap(ui string) map[string]string {
	info := make(map[string]string)
	if ui == "" {
		return info
	}

	data := strings.Split(ui[1:], "\\")

	for i := 0; i < len(data); i += 2 {
		info[data[i]] = data[i+1]
	}

	// special case: split the IP value into IP and Port
	ip := info["ip"]
	ipport := strings.Split(ip, ":")
	if len(ipport) >= 2 {
		info["port"] = ipport[1]
		info["ip"] = ipport[0]
	}

	return info
}

/**
 * Locate the struct of the server for a particular
 * ID, get a pointer to it
 */
func findserver(lookup string) (*Server, error) {
	for i, srv := range servers {
		if srv.uuid == lookup {
			return &servers[i], nil
		}
	}

	return nil, errors.New("unknown server")
}

/**
 * Send all messages in the outgoing queue to the gameserver
 */
func SendMessages(srv *Server) {
	if !srv.connected {
		return
	}

	// key have been exchanged, encrypt the message
	if srv.trusted && srv.encrypted {
		cipher := SymmetricEncrypt(
			srv.aeskey,
			srv.aesiv,
			srv.messageout.buffer[:srv.messageout.length])

		clearmsg(&srv.messageout)
		srv.messageout.buffer = cipher
		srv.messageout.length = len(cipher)
	}

	if srv.messageout.length > 0 {
		if config.Debug == 1 {
			fmt.Printf("Sending\n%s\n\n", hex.Dump(srv.messageout.buffer))
		}
		(*srv.connection).Write(srv.messageout.buffer)
		clearmsg(&srv.messageout)
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

	//fmt.Printf("Read Input:\n%s\n\n", hex.Dump(input[0:bytesread]))

	magic := ReadLong(&msg)
	if magic != 1128346193 {
		// not a valid client, just close connection
		c.Close()
		return
	}
	log.Println("Magic value accepted")

	_ = ReadByte(&msg) // should be CMDHello
	uuid := ReadString(&msg)
	ver := ReadLong(&msg)
	port := ReadShort(&msg)
	maxplayers := ReadByte(&msg)
	enc := ReadByte(&msg)
	clNonce := ReadData(&msg, challengeLength)

	if ver < versionRequired {
		c.Close()
		return
	}
	log.Println("Running acceptable version")

	server, err := findserver(uuid)
	if err != nil {
		// write an error, close socket, returns
		log.Println(err)
		c.Close()
		return
	}
	log.Printf("Server located: %s\n", server.name)

	server.port = int(port)
	server.encrypted = int(enc) == 1 // stupid bool conversion
	server.connection = &c
	server.connected = true
	server.version = int(ver)
	server.maxplayers = int(maxplayers)
	keyname := fmt.Sprintf("keys/%s.pem", uuid)

	log.Printf("Loading public key: %s\n", keyname)
	pubkey, err := LoadPublicKey(keyname)
	if err != nil {
		log.Printf("Public key error: %s\n", err.Error())
		c.Close()
		return
	}
	server.publickey = pubkey

	challengeCipher := Sign(q2a.privatekey, clNonce)
	WriteByte(SCMDHelloAck, &server.messageout)
	WriteShort(len(challengeCipher), &server.messageout)
	WriteData(challengeCipher, &server.messageout)

	/**
	 * if client requests encrypted transit, encrypt the session key/iv
	 * with the client's public key to keep it confidential
	 */
	if server.encrypted {
		server.aeskey = RandomBytes(AESBlockLength)
		server.aesiv = RandomBytes(AESIVLength)
		blob := append(server.aeskey, server.aesiv...)
		aescipher := PublicEncrypt(server.publickey, blob)
		WriteData(aescipher, &server.messageout)
	}

	svchallenge := RandomBytes(challengeLength)
	WriteData(svchallenge, &server.messageout)

	//c.Write(server.messageout.buffer)
	SendMessages(server)

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
	verified := VerifySignature(server.publickey, svchallenge, clientSignature)

	if verified {
		log.Printf("%s signature verified\n", server.name)
	} else {
		log.Printf("%s signature verifcation failed...", server.name)
		c.Close()
		return
	}

	LoadBans(server)
	WriteByte(SCMDTrusted, &server.messageout)
	SendMessages(server)
	server.trusted = true

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
		if server.encrypted && server.trusted {
			fmt.Printf("Cipher Read:\n%s\n", hex.Dump(input[:size]))
			input, size = SymmetricDecrypt(server.aeskey, server.aesiv, input)
			fmt.Printf("Clear Read:\n%s\n", hex.Dump(input[:size]))
		}

		server.message.buffer = input
		server.message.index = 0
		server.message.length = size
		//server.message.length = size

		//fmt.Printf("Read:\n%s\n\n", hex.Dump(input[:size]))
		ParseMessage(server)
		SendMessages(server)
	}

	server.connected = false
	server.trusted = false
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

/**
 * pre-entry point
 */
func init() {
	configfile := "q2a.json" // override with cli arg
	if len(os.Args) > 1 {
		configfile = os.Args[1]
	}

	log.Printf("Loading config from %s\n", configfile)
	confjson, err := os.ReadFile(configfile)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(confjson, &config)
	if err != nil {
		log.Fatal(err)
	}

	rand.Seed(time.Now().Unix())

	log.Printf("Loading private key %s\n", config.PrivateKey)
	privkey, err := LoadPrivateKey(config.PrivateKey)
	if err != nil {
		log.Fatalf("Problems loading private key: %s\n", err.Error())
	}

	pubkey := privkey.Public().(*rsa.PublicKey)
	q2a.privatekey = privkey
	q2a.publickey = pubkey

	LoadGlobalBans()

	db = DatabaseConnect()

	log.Println("Loading servers:")
	servers = LoadServers(db)
	for _, s := range servers {
		log.Printf("  %-15s %-21s [%s]", s.name, fmt.Sprintf("%s:%d", s.ipaddress, s.port), s.uuid)
	}
}

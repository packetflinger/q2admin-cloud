package main

import (
	//"bufio"
	"database/sql"
	"os/signal"

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
 * Use a custom buffer struct to keep track of where
 * we are in the stream of bytes internally
 */
type MessageBuffer struct {
	buffer []byte
	index  int
	length int // maybe not needed
}

/**
 * This is a Quake 2 Gameserver, and also a client to us.
 *
 * This struct is partially populated by the database on
 * init and the rest is filled in when the game server
 * actually connects
 */
type Server struct {
	id         int // this is the database index
	uuid       string
	owner      int // user id from database
	version    int // what version are we running
	name       string
	ipaddress  string // used for teleporting
	port       int    // used for teleporting
	connected  bool   // is it currently connected to us?
	currentmap string
	enabled    bool
	connection *net.Conn
	players    []Player
	maxplayers int
	message    MessageBuffer  // incoming byte stream
	messageout MessageBuffer  // outgoing byte stream
	encrypted  bool           // are the messages AES encrypted?
	trusted    bool           // signature challenge verified
	publickey  *rsa.PublicKey // supplied by owner via website
	aeskey     []byte         // 16 (128bit)
	aesiv      []byte         // 16 bytes (CBC)
	bans       []Ban
	pingcount  int
}

/**
 * "This" admin server
 */
type AdminServer struct {
	privatekey *rsa.PrivateKey
	publickey  *rsa.PublicKey
}

/**
 * The config file once parsed
 */
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

	// keys have been exchanged, encrypt the message
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
		(*srv.connection).Write(srv.messageout.buffer)
		clearmsg(&srv.messageout)
	}
}

/**
 * Dates are stored in the database as unix timestamps
 */
func GetUnixTimestamp() int64 {
	return time.Now().Unix()
}

/**
 * Get a time "object" from a database timestamp
 */
func GetTimeFromTimestamp(ts int64) time.Time {
	return time.Unix(ts, 0)
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
	log.Printf("[%s] connecting...\n", server.name)

	server.port = int(port)
	server.encrypted = int(enc) == 1 // stupid bool conversion
	server.connection = &c
	server.connected = true
	server.version = int(ver)
	server.maxplayers = int(maxplayers)
	keyname := fmt.Sprintf("keys/%s.pem", uuid)

	log.Printf("[%s] Loading public key: %s\n", server.name, keyname)
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
		log.Printf("[%s] signature verified, server trusted\n", server.name)
	} else {
		log.Printf("[%s] signature verifcation failed...", server.name)
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
			input, size = SymmetricDecrypt(server.aeskey, server.aesiv, input[:size])
		}

		server.message.buffer = input
		server.message.index = 0
		server.message.length = size

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

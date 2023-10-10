package server

import (
	"crypto/rsa"
	"database/sql"
	"errors"

	"fmt"
	"log"
	"net"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/database"
	pb "github.com/packetflinger/q2admind/proto"
)

// "This" admin server
type RemoteAdminServer struct {
	Users      []*pb.User      // website users
	Config     pb.Config       // global config
	Clients    []client.Client // managed quake 2 servers
	Rules      []*pb.Rule      // bans/mutes/etc
	Privatekey *rsa.PrivateKey // private to us
	Publickey  *rsa.PublicKey  // known to clients
	MaintCount int             // total maintenance runs
}

var (
	Q2A RemoteAdminServer // this server
	DB  *sql.DB
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

/*
// Initialize a message buffer
func clearmsg(msg *util.MessageBuffer) {
	msg.buffer = nil
	msg.index = 0
	msg.length = 0
}
*/

// Locate the struct of the server for a particular
// ID, get a pointer to it
func FindClient(lookup string) (*client.Client, error) {
	for i, cl := range Q2A.Clients {
		if cl.UUID == lookup {
			return &Q2A.Clients[i], nil
		}
	}

	return nil, errors.New("unknown client")
}

// Send the txt string to all the websockets listening
func (cl *client.Client) SendToWebsiteFeed(txt string, decoration int) {
	/*
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
	*/
}

// Setup the connection
// The first message sent should identify the game server
// and trigger the authentication process. Connection
// persists in a goroutine from this function.
//
// Called from main loop when a new connection is made
func HandleConnection(c net.Conn) {
	log.Printf("Serving %s\n", c.RemoteAddr().String())

	input := make([]byte, 5000)
	_, _ = c.Read(input)
	msg := message.NewMessageBuffer(input)

	magic := msg.ReadLong()
	if magic != ProtocolMagic {
		// not a valid client, just close connection
		log.Println("Bad magic value in new connection, not a valid client")
		c.Close()
		return
	}

	_ = msg.ReadByte() // should be CMDHello
	uuid := msg.ReadString()
	ver := msg.ReadLong()
	port := msg.ReadShort()
	maxplayers := msg.ReadByte()
	enc := msg.ReadByte()
	clNonce := msg.ReadData(challengeLength)

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
	pubkey, err := crypto.LoadPublicKey(keyname)
	if err != nil {
		log.Printf("Public key error: %s\n", err.Error())
		c.Close()
		return
	}
	cl.PublicKey = pubkey

	challengeCipher := crypto.Sign(Q2A.Privatekey, clNonce)
	WriteByte(SCMDHelloAck, &cl.MessageOut)
	WriteShort(len(challengeCipher), &cl.MessageOut)
	WriteData(challengeCipher, &cl.MessageOut)

	// If client requests encrypted transit, encrypt the session key/iv
	// with the client's public key to keep it confidential
	if cl.Encrypted {
		cl.AESKey = crypto.RandomBytes(AESBlockLength)
		cl.AESIV = crypto.RandomBytes(AESIVLength)
		blob := append(cl.AESKey, cl.AESIV...)
		aescipher := crypto.PublicEncrypt(cl.PublicKey, blob)
		WriteData(aescipher, &cl.MessageOut)
	}

	svchallenge := crypto.RandomBytes(challengeLength)
	WriteData(svchallenge, &cl.MessageOut)

	cl.SendMessages()

	// read the client signature
	_, _ = c.Read(input)
	msg = message.NewMessageBuffer(input)

	op := msg.ReadByte() // should be CMDAuth (0x0d)
	if op != CMDAuth {
		c.Close()
		return
	}

	sigsize := msg.ReadShort()
	clientSignature := msg.ReadData(int(sigsize))
	verified := crypto.VerifySignature(cl.PublicKey, svchallenge, clientSignature)

	if verified {
		log.Printf("[%s] signature verified, server trusted\n", cl.Name)
		cl.LogEvent("connected")
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
			input, size = crypto.SymmetricDecrypt(cl.AESKey, cl.AESIV, input[:size])
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
	cl.LogEvent("disconnected")
}

// Gracefully shut everything down
func Shutdown() {
	log.Println("Shutting down...")
	//LogSystemEvent("shutdown")
	DB.Close() // not sure if this is necessary
}

// Start the cloud admin server
func Startup() {
	log.Println("Loading private key:", Q2A.Config.GetPrivateKey())
	privkey, err := crypto.LoadPrivateKey(Q2A.Config.GetPrivateKey())
	if err != nil {
		log.Fatalf("Problems loading private key: %s\n", err.Error())
	}

	pubkey := privkey.Public().(*rsa.PublicKey)
	Q2A.Privatekey = privkey
	Q2A.Publickey = pubkey

	DB = database.DatabaseConnect()

	rules, err := FetchRules("config/rules")
	if err != nil {
		log.Println(err)
	} else {
		Q2A.Rules = rules
	}

	log.Println("Loading clients from:", Q2A.Config.GetClientFile())
	clients, err := client.LoadClients(Q2A.Config.GetClientFile())
	if err != nil {
		log.Println(err)
	} else {
		Q2A.Clients = clients
	}

	// Read users
	log.Println("Loading users from:", Q2A.Config.GetUserFile())
	users, err := api.ReadUsersFromDisk(Q2A.Config.GetUserFile())
	if err != nil {
		log.Println(err)
	} else {
		Q2A.Users = users
	}

	for _, c := range Q2A.Clients {
		log.Printf("server: %-25s [%s:%d]", c.Name, c.IPAddress, c.Port)
	}

	port := fmt.Sprintf("%s:%d", Q2A.Config.Address, Q2A.Config.Port)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		fmt.Println(err)
		return
	}

	defer listener.Close()

	log.Printf("Listening for gameservers on %s\n", port)

	if Q2A.Config.GetApiEnabled() {
		//go api.RunHTTPServer()
	}

	go startMaintenance()

	for {
		c, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go HandleConnection(c)
	}
}

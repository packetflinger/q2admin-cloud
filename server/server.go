package server

import (
	"crypto/rsa"
	"database/sql"
	"errors"
	"os"

	"fmt"
	"log"
	"net"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/database"
	pb "github.com/packetflinger/q2admind/proto"
	"google.golang.org/protobuf/encoding/prototext"
)

// "This" admin server
type CloudAdminServer struct {
	Users      []*pb.User      // website users
	Config     pb.Config       // global config
	Clients    []client.Client // managed quake 2 servers
	Rules      []*pb.Rule      // bans/mutes/etc
	Privatekey *rsa.PrivateKey // private to us
	Publickey  *rsa.PublicKey  // known to clients
	MaintCount int             // total maintenance runs
}

var (
	Cloud CloudAdminServer // this server
	DB    *sql.DB
)

const (
	ProtocolMagic   = 1128346193 // "Q2AC"
	versionRequired = 342        // git revision number
	challengeLength = 16         // bytes
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

// Locate the struct of the server for a particular
// ID, get a pointer to it
func FindClient(lookup string) (*client.Client, error) {
	for i, cl := range Cloud.Clients {
		if cl.UUID == lookup {
			return &Cloud.Clients[i], nil
		}
	}

	return nil, errors.New("unknown client")
}

// Get a pointer to a user based on their email
func GetUserByEmail(email string) (*pb.User, error) {
	for _, u := range Cloud.Users {
		if u.GetEmail() == email {
			return u, nil
		}
	}
	return &pb.User{}, errors.New("user not found")
}

// Someone deleted a managed server via the web interface.
// This should mean:
// - remove from database, including foreign key constraints
// - close any open connections to this server
// - remove from active server slice in memory
//
// TODO: make this better
func RemoveClient(uuid string) bool {
	cl, err := FindClient(uuid)
	if err != nil {
		return false
	}

	// mark in-ram server object as disabled to prevent reconnects
	cl.Enabled = false

	tr, err := DB.Begin()
	if err != nil {
		log.Println(err)
		return false
	}

	sql := "DELETE FROM server WHERE id = ?"
	_, err = tr.Exec(sql, cl.ID)
	if err != nil {
		log.Println(err)
		tr.Rollback()
		return false
	}

	// log data?
	// chat data?

	tr.Commit()
	return true
}

// Change symmetric keys. Generate new key and iv and
// immediately send them to the client. This jumps ahead
// of the normal send buffer so that all messages from
// this point on can be decrypted on the client.
//
// Called from Pong() every hour or so
func RotateKeys(cl *client.Client) {
	if !cl.Encrypted {
		return
	}

	key := crypto.RandomBytes(crypto.AESBlockLength)
	iv := crypto.RandomBytes(crypto.AESIVLength)
	blob := append(key, iv...)

	// Send immediately so old keys used for this message
	(&cl.MessageOut).WriteByte(SCMDKey)
	(&cl.MessageOut).WriteData(blob)
	cl.SendMessages()

	cl.AESKey = key
	cl.AESIV = iv
}

// Write the clients proto to disk as text-format
func WriteClients(outfile string, clients []client.Client) error {
	clientspb := []*pb.Clients_Client{}
	for _, c := range clients {
		p := c.ToProto()
		clientspb = append(clientspb, p)
	}

	// combine into a single message
	cls := pb.Clients{
		Client: clientspb,
	}

	opt := prototext.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}
	textpb, err := opt.Marshal(&cls)
	if err != nil {
		return err
	}
	err = os.WriteFile(outfile, textpb, 0777)
	if err != nil {
		return err
	}
	return nil
}

// Setup the connection
// The first message sent should identify the game server
// and trigger the authentication process. Connection
// persists in a goroutine from this function.
//
// Called from main loop when a new connection is made
func HandleConnection(c net.Conn) {
	defer c.Close()
	log.Printf("Serving %s\n", c.RemoteAddr().String())

	input := make([]byte, 5000)
	readlen, err := c.Read(input)
	if err != nil {
		log.Println("Client read error:", err)
		return
	}
	if readlen != 50+challengeLength {
		log.Printf("Invalid hello length - got %d, want %d\n", readlen, 50+challengeLength)
		return
	}
	msg := message.NewMessageBuffer(input)

	if msg.ReadLong() != ProtocolMagic {
		log.Println("Bad magic value in new connection, not a valid client")
		return
	}

	if msg.ReadByte() != CMDHello {
		log.Println("Protocol error: expecting CMDHello, closing connection")
		return
	}
	uuid := msg.ReadString()
	ver := msg.ReadLong()
	port := msg.ReadShort()
	maxplayers := msg.ReadByte()
	enc := msg.ReadByte()
	clNonce := msg.ReadData(challengeLength)

	if ver < versionRequired {
		log.Printf("Old client - got version %d, want at least %d\n", ver, versionRequired)
		return
	}

	cl, err := FindClient(uuid)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("[%s] connecting...\n", cl.Name)

	cl.Port = int(port)
	cl.Encrypted = int(enc) == 1 // stupid bool conversion
	cl.Connection = &c
	cl.Connected = true
	cl.Version = int(ver)
	cl.MaxPlayers = int(maxplayers)

	keyFile := fmt.Sprintf("clients/%s/key", cl.Name)

	log.Printf("[%s] Loading public key: %s\n", cl.Name, keyFile)
	pubkey, err := crypto.LoadPublicKey(keyFile)
	if err != nil {
		log.Printf("Public key error: %s\n", err.Error())
		return
	}
	cl.PublicKey = pubkey

	challengeCipher := crypto.Sign(Cloud.Privatekey, clNonce)

	out := &cl.MessageOut
	out.WriteByte(SCMDHelloAck)
	out.WriteShort(uint16(len(challengeCipher)))
	out.WriteData(challengeCipher)

	// If client requests encrypted transit, encrypt the session key/iv
	// with the client's public key to keep it confidential
	if cl.Encrypted {
		cl.AESKey = crypto.RandomBytes(crypto.AESBlockLength)
		cl.AESIV = crypto.RandomBytes(crypto.AESIVLength)
		blob := append(cl.AESKey, cl.AESIV...)
		aescipher := crypto.PublicEncrypt(cl.PublicKey, blob)
		out.WriteData(aescipher)
	}

	svchallenge := crypto.RandomBytes(challengeLength)
	out.WriteData(svchallenge)

	cl.SendMessages()

	// read the client signature
	readlen, err = c.Read(input)
	if err != nil {
		log.Println("Error reading client auth response:", err)
		return
	}

	// We're using a 256bit hashing algo for signing, so we should read
	// at least 32 + 3 (command bit + length) bytes
	if readlen < 35 {
		log.Printf("Invalid client auth length read - got %d, want at least 35\n", readlen)
		return
	}
	msg = message.NewMessageBuffer(input)

	op := msg.ReadByte() // should be CMDAuth (0x0d)
	if op != CMDAuth {
		log.Printf("Protocol auth error - got %d, want %d\n", op, CMDAuth)
		return
	}

	sigsize := msg.ReadShort()
	clientSignature := msg.ReadData(int(sigsize))
	verified := crypto.VerifySignature(cl.PublicKey, svchallenge, clientSignature)

	if !verified {
		log.Printf("[%s] signature verifcation failed...", cl.Name)
		return
	}

	log.Printf("[%s] authenticated\n", cl.Name)
	out.WriteByte(SCMDTrusted)
	cl.SendMessages()
	cl.Trusted = true

	cl.Players = make([]client.Player, cl.MaxPlayers)

	// main connection loop for this client
	// - wait for input
	// - parse any messages received, react as necessary
	// - send any responses
	for {
		input := make([]byte, 5000)
		size, err := c.Read(input)
		if err != nil {
			log.Printf("[%s] read error (disconnecting): %s\n", cl.Name, err.Error())
			break
		}

		if cl.Encrypted && cl.Trusted {
			input, _ = crypto.SymmetricDecrypt(cl.AESKey, cl.AESIV, input[:size])
		}

		cl.Message = message.NewMessageBuffer(input)

		ParseMessage(cl)
		cl.SendMessages()
	}

	cl.Connected = false
	cl.Trusted = false
}

// Gracefully shut everything down
//
// Close database connection, write states to disk, etc
func Shutdown() {
	fmt.Println("")
	log.Println("Shutting down...")
	DB.Close() // not sure if this is necessary
}

// Start the cloud admin server
func Startup() {
	log.Println("Loading private key:", Cloud.Config.GetPrivateKey())
	privkey, err := crypto.LoadPrivateKey(Cloud.Config.GetPrivateKey())
	if err != nil {
		log.Fatalf("Problems loading private key: %s\n", err.Error())
	}

	pubkey := privkey.Public().(*rsa.PublicKey)
	Cloud.Privatekey = privkey
	Cloud.Publickey = pubkey

	DB = database.DatabaseConnect(Cloud.Config.Database)

	rules, err := FetchRules("config/rules")
	if err != nil {
		log.Println(err)
	} else {
		Cloud.Rules = rules
	}

	log.Println("Loading clients from:", Cloud.Config.GetClientFile())
	clients, err := client.LoadClients(Cloud.Config.GetClientFile())
	if err != nil {
		log.Println(err)
	} else {
		Cloud.Clients = clients
	}

	// Read users
	log.Println("Loading users from:", Cloud.Config.GetUserFile())
	users, err := api.ReadUsersFromDisk(Cloud.Config.GetUserFile())
	if err != nil {
		log.Println(err)
	} else {
		Cloud.Users = users
	}

	for _, c := range Cloud.Clients {
		log.Printf("server: %-25s [%s:%d]", c.Name, c.IPAddress, c.Port)
	}

	port := fmt.Sprintf("%s:%d", Cloud.Config.Address, Cloud.Config.Port)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		log.Println(err)
		return
	}

	defer listener.Close()

	log.Printf("Listening for gameservers on %s\n", port)

	if Cloud.Config.GetApiEnabled() {
		creds, err := ReadOAuthCredsFromDisk(Cloud.Config.GetAuthFile())
		if err != nil {
			log.Println(err)
		}
		go RunHTTPServer(Cloud.Config.GetApiAddress(), int(Cloud.Config.GetApiPort()), creds)
	}

	go startMaintenance()

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Println(err)
			return
		}
		go HandleConnection(c)
	}
}

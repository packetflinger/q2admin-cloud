package server

import (
	"bytes"
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/database"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// "This" admin server
type Server struct {
	users      []*pb.User      // website users
	config     pb.Config       // global config
	clients    []client.Client // managed quake 2 servers
	rules      []*pb.Rule      // bans/mutes/etc
	privateKey *rsa.PrivateKey // private to us
	publicKey  *rsa.PublicKey  // known to clients
	maintCount int             // total maintenance runs
	// ipCache    map[string]IPInfo // key is the IP address
}

// Information about a particular IP address, including any PTR records from DNS,
// and whether it's associated with a VPN provider.
type IPInfo struct {
	Addr       string
	Hostname   string
	TimeToLive int64
	VPN        bool
	Lookups    int64
}

var (
	srv Server // this server
	db  database.Database
)

const (
	ProtocolMagic   = 1128346193 // "Q2AC"
	versionRequired = 706        // git revision number
	challengeLength = 16         // bytes
	TeleportWidth   = 80         // max chars per line for teleport replies
	StifleMax       = 300        // 5 minutes
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

const (
	LogLevelNormal    = iota // operational stuff
	LogLevelInfo             // more detail
	LogLevelDebug            // a lot of detail
	LogLevelDeveloper        // meaningless to all but devs
	LogLevelAll              // everything
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
func (s *Server) FindClient(lookup string) (*client.Client, error) {
	for i := range s.clients {
		if s.clients[i].UUID == lookup {
			return &s.clients[i], nil
		}
	}
	return nil, errors.New("unknown client")
}

// Locate the struct of the server for a particular
// name, get a pointer to it
func (s *Server) FindClientByName(name string) (*client.Client, error) {
	for i := range s.clients {
		if s.clients[i].Name == name {
			return &s.clients[i], nil
		}
	}
	return nil, errors.New("unknown client")
}

// Get a pointer to a user based on their email
func (s *Server) GetUserByEmail(email string) (*pb.User, error) {
	for _, u := range s.users {
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
	cl, err := srv.FindClient(uuid)
	if err != nil {
		return false
	}

	// mark in-ram server object as disabled to prevent reconnects
	cl.Enabled = false

	tr, err := db.Begin()
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

// ClientsByContext will provide a collection of pointers for clients
// accessible to the context.
//
// Circular: find clients by context to include in that context
func ClientsByContext(ctx *IdentityContext) []*client.Client {
	cls := []*client.Client{}
	for i, cl := range srv.clients {
		if cl.Owner == ctx.user.Email {
			cls = append(cls, &srv.clients[i])
			continue
		}
		for _, key := range cl.APIKeys.GetKey() {
			if key.GetSecret() == ctx.apiKey {
				cls = append(cls, &srv.clients[i])
			}
		}
	}
	return cls
}

// Acquire a slice of client pointers that a particular identity
// has access to (owners and delegates)
func ClientsByIdentity(ident string) []client.Client {
	list := []client.Client{}
	for _, cl := range srv.clients {
		if strings.EqualFold(cl.Owner, ident) {
			list = append(list, cl)
		}
	}
	return list
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

	keyData := crypto.EncryptionKey{
		Key:        crypto.RandomBytes(crypto.AESBlockLength),
		InitVector: crypto.RandomBytes(crypto.AESIVLength),
	}
	blob := append(keyData.Key, keyData.InitVector...)

	// Send immediately so old keys used for this message
	(&cl.MessageOut).WriteByte(SCMDKey)
	(&cl.MessageOut).WriteData(blob)
	SendMessages(cl)

	cl.CryptoKey = keyData
}

// Read all client names from disk, load their data
// into memory. Add each to the client list.
//
// Called from initialize() at startup
func LoadClients(filename string) ([]client.Client, error) {
	clients := []client.Client{}
	clientspb := pb.ClientList{}

	contents, err := os.ReadFile(filename)
	if err != nil {
		return clients, err
	}
	err = prototext.Unmarshal(contents, &clientspb)
	if err != nil {
		return clients, err
	}

	clientNames := clientspb.GetClient()
	for _, c := range clientNames {
		cl, err := client.LoadSettings(c, srv.config.ClientDirectory)
		if err != nil {
			continue
		}
		cl.Rules, err = cl.FetchRules()
		if err != nil {
			log.Println(err)
		}
		clients = append(clients, cl)
	}
	return clients, nil
}

// Write the clients proto to disk as text-format
func WriteClients(outfile string, clients []client.Client) error {
	clientspb := []*pb.Client{}
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
func (s *Server) HandleConnection(c net.Conn) {
	defer c.Close()

	err := c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		s.Logf(LogLevelNormal, "error setting read deadline: %v\n", err)
		return
	}

	srv.Logf(LogLevelNormal, "serving %s\n", c.RemoteAddr().String())

	input := make([]byte, 5000)
	readlen, err := c.Read(input)
	if err != nil {
		srv.Logf(LogLevelNormal, "Client read error: %v\n", err)
		return
	}
	if readlen != 50+crypto.RSAKeyLength {
		srv.Logf(LogLevelDeveloper, "Invalid hello length - got %d, want %d\n", readlen, 50+crypto.RSAKeyLength)
		return
	}
	msg := message.NewMessageBuffer(input)

	if msg.ReadLong() != ProtocolMagic {
		srv.Logf(LogLevelNormal, "invalid client\n")
		return
	}

	if msg.ReadByte() != CMDHello {
		srv.Logf(LogLevelNormal, "bad message type, closing connection")
		return
	}
	uuid := msg.ReadString()
	ver := msg.ReadLong()
	port := msg.ReadShort()
	maxplayers := msg.ReadByte()
	enc := msg.ReadByte()
	challenge := msg.ReadData(crypto.RSAKeyLength)
	clNonce, err := crypto.PrivateDecrypt(srv.privateKey, challenge)
	if err != nil {
		log.Println(err)
		log.Println("key mismatch?")
		return
	}
	hash, err := crypto.MessageDigest(clNonce)
	if err != nil {
		log.Println(err)
		return
	}

	cl, err := srv.FindClient(uuid)
	if err != nil {
		log.Println(err)
		return
	}
	cl.Path = path.Join(srv.config.GetClientDirectory(), cl.Name)

	cl.Log, err = NewClientLogger(cl)
	if err != nil {
		srv.Logf(LogLevelNormal, "[%s] error creating logger: %v\n", cl.Name, err)
	}
	cl.Log.Printf("[%s] connecting...\n", cl.IPAddress)

	if ver < versionRequired {
		srv.Logf(LogLevelNormal, "Old client - got version %d, want at least %d\n", ver, versionRequired)
		cl.Log.Printf("q2admin library too old - found version %d, need at least %d\n", ver, versionRequired)
		return
	}

	cl.TermLog = make(chan string)

	cl.Port = int(port)
	cl.Encrypted = int(enc) == 1
	cl.Connection = &c
	cl.Connected = true
	cl.Version = int(ver)
	cl.MaxPlayers = int(maxplayers)

	keyFile := path.Join(srv.config.ClientDirectory, cl.Name, "key")

	srv.Logf(LogLevelInfo, "[%s] Loading public key: %s\n", cl.Name, keyFile)
	pubkey, err := crypto.LoadPublicKey(keyFile)
	if err != nil {
		srv.Logf(LogLevelNormal, "error loading public key: %v\n", err)
		return
	}
	cl.PublicKey = pubkey

	svNonce := crypto.RandomBytes(16)
	blob := append(hash, svNonce...)

	// If client requests encrypted transit, encrypt the session key/iv
	// with the client's public key to keep it confidential
	if cl.Encrypted {
		cl.CryptoKey = crypto.EncryptionKey{
			Key:        crypto.RandomBytes(crypto.AESBlockLength),
			InitVector: crypto.RandomBytes(crypto.AESIVLength),
		}
		blob = append(blob, cl.CryptoKey.Key...)
		blob = append(blob, cl.CryptoKey.InitVector...)
	}

	blobCipher := crypto.PublicEncrypt(cl.PublicKey, blob)

	out := &cl.MessageOut
	out.WriteByte(SCMDHelloAck)
	out.WriteShort(uint16(len(blobCipher)))
	out.WriteData(blobCipher)
	SendMessages(cl)

	// read the client signature
	readlen, err = c.Read(input)
	if err != nil {
		srv.Logf(LogLevelNormal, "error reading client auth response: %v\n", err)
		return
	}

	// We're using a 256bit hashing algo for signing, so we should read
	// at least 32 + 3 (command bit + length) bytes
	if readlen < 35 {
		srv.Logf(LogLevelDebug, "invalid client auth length read - got %d, want at least 35\n", readlen)
		return
	}
	msg = message.NewMessageBuffer(input)

	op := msg.ReadByte() // should be CMDAuth (0x0d)
	if op != CMDAuth {
		srv.Logf(LogLevelDebug, "Protocol auth error - got %d, want %d\n", op, CMDAuth)
		return
	}

	authLen := msg.ReadShort()
	authCipher := msg.ReadData(int(authLen))
	authMD, err := crypto.PrivateDecrypt(srv.privateKey, authCipher)
	if err != nil {
		msg := fmt.Sprintf("private key error: %v", err)
		srv.Logf(LogLevelNormal, msg)
	}
	authHash, err := crypto.MessageDigest(svNonce)
	if err != nil {
		srv.Logf(LogLevelInfo, "[%s] hashing error: %v\n", cl.Name, err)
	}

	verified := false
	if bytes.Equal(authHash, authMD) {
		verified = true
	}

	if !verified {
		srv.Logf(LogLevelNormal, "[%s] auth failed\n", cl.Name)
		cl.Log.Println("authentication failed, disconnecting")
		return
	}

	srv.Logf(LogLevelNormal, "[%s] authenticated\n", cl.Name)
	cl.Log.Println("authenticated")

	out.WriteByte(SCMDTrusted)
	SendMessages(cl)
	cl.Trusted = true

	cl.Players = make([]client.Player, cl.MaxPlayers)

	// main connection loop for this client
	// - wait for input
	// - parse any messages received, react as necessary
	// - send any responses
	var inputSize int
	for {
		input := make([]byte, 5000)
		size, err := c.Read(input)
		if err != nil {
			srv.Logf(LogLevelInfo, "[%s] read error: %v\n", cl.Name, err)
			cl.Log.Println("read error:", err)
			break
		}

		if cl.Encrypted && cl.Trusted {
			input, inputSize = crypto.SymmetricDecrypt(cl.CryptoKey.Key, cl.CryptoKey.InitVector, input[:size])
			if inputSize == 0 {
				srv.Logf(LogLevelNormal, "[%s] symmetric decrypt error\n", cl.Name)
				cl.Log.Println("decryption error, dropping client")
				break
			}
		}

		cl.Message = message.NewMessageBuffer(input[:size])

		ParseMessage(cl)
		SendMessages(cl)
	}

	cl.Connected = false
	cl.Trusted = false
}

// Send all messages in the outgoing queue to the client (gameserver)
func SendMessages(cl *client.Client) {
	if !cl.Connected {
		return
	}

	if len(cl.MessageOut.Buffer) == 0 {
		return
	}

	// keys have been exchanged, encrypt the message
	if cl.Trusted && cl.Encrypted {
		cipher := crypto.SymmetricEncrypt(
			cl.CryptoKey.Key,
			cl.CryptoKey.InitVector,
			cl.MessageOut.Buffer[:cl.MessageOut.Index])
		cl.MessageOut = message.NewMessageBuffer(cipher)
	}

	// only send if there is something to send
	if len(cl.MessageOut.Buffer) > 0 {
		(*cl.Connection).Write(cl.MessageOut.Buffer)
		(&cl.MessageOut).Reset()
	}
}

// Gracefully shut everything down
//
// Close database connection, write states to disk, etc
func Shutdown() {
	srv.Logf(LogLevelNormal, "Shutting down...")
	db.Handle.Close() // not sure if this is necessary
}

// context logging for server. Will output the date/time, source file name and
// line number, and a formatted string. Logging is dependant on verbosity level
// from the config.
func (s *Server) Logf(level int, format string, args ...any) {
	if int(s.config.GetVerboseLevel()) < level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	_, src, line, ok := runtime.Caller(1) // from parent, not here
	if ok && s.config.GetVerboseLevel() > LogLevelNormal {
		log.Printf("%s:%d] %s", path.Base(src), line, msg)
		return
	}
	log.Print(msg)
}

// Start the cloud admin server
func Startup(configFile string, foreground bool) {
	log.Printf("%-21s %s\n", "Loading config:", configFile)
	textpb, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = prototext.Unmarshal(textpb, &srv.config)
	if err != nil {
		log.Fatal(err)
	}

	if !foreground {
		f, err := os.OpenFile(srv.config.GetLogFile(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	srv.Logf(LogLevelInfo, "%-21s %s\n", "loading private key:", srv.config.GetPrivateKey())
	privkey, err := crypto.LoadPrivateKey(srv.config.GetPrivateKey())
	if err != nil {
		log.Fatalf("error loading private key: %v\n", err)
	}

	pubkey := privkey.Public().(*rsa.PublicKey)
	srv.privateKey = privkey
	srv.publicKey = pubkey

	srv.Logf(LogLevelInfo, "%-21s %s\n", "opening database:", srv.config.Database)
	db, err = database.Open(srv.config.Database)
	if err != nil {
		log.Println(err)
		return
	}

	srv.Logf(LogLevelInfo, "%-21s %s\n", "loading global rules:", srv.config.GetRuleFile())
	rules, err := FetchRules(srv.config.GetRuleFile())
	if err != nil {
		log.Println(err)
	} else {
		srv.rules = rules
	}

	// Read users
	srv.Logf(LogLevelInfo, "%-21s %s\n", "loading users:", srv.config.GetUserFile())
	users, err := api.ReadUsersFromDisk(srv.config.GetUserFile())
	if err != nil {
		log.Println(err)
	} else {
		srv.users = users
	}

	srv.Logf(LogLevelNormal, "%-21s %s\n", "loading clients:", srv.config.GetClientFile())
	clients, err := LoadClients(srv.config.GetClientFile())
	if err != nil {
		log.Println(err)
	} else {
		slices.SortFunc(clients, func(a, b client.Client) int {
			return strings.Compare(a.Name, b.Name)
		})
		srv.clients = clients
		for _, c := range srv.clients {
			srv.Logf(LogLevelNormal, "  %-25s [%s:%d]", c.Name, c.IPAddress, c.Port)
		}
	}

	port := fmt.Sprintf("%s:%d", srv.config.Address, srv.config.Port)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		log.Println(err)
		return
	}

	defer listener.Close()

	srv.Logf(LogLevelNormal, "listening for gameservers on %s\n", port)

	if srv.config.GetApiEnabled() {
		creds, err := ReadOAuthCredsFromDisk(srv.config.GetAuthFile())
		if err != nil {
			log.Println(err)
		}
		go srv.RunHTTPServer(srv.config.GetApiAddress(), int(srv.config.GetApiPort()), creds)
	}

	go srv.startMaintenance()
	go srv.startManagement()
	go srv.startSSHServer()

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Println(err)
			return
		}
		go srv.HandleConnection(c)
	}
}

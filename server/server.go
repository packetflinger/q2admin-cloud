package server

import (
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
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
}

var (
	srv Server // this server
	db  database.Database
)

const (
	ProtocolMagic       = 1128346193 // "Q2AC"
	versionRequired     = 706        // git revision number
	challengeLength     = 16         // bytes
	TeleportWidth       = 80         // max chars per line for teleport replies
	StifleMax           = 300        // 5 minutes
	GreetingLength      = 306
	NetReadLength       = 5000
	MaxInviteTokens     = 3
	InviteTokenInterval = 300 // seconds per token added
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
	LogLevelNormal        = iota // operational stuff
	LogLevelInfo                 // more detail
	LogLevelDebug                // a lot of detail
	LogLevelDeveloper            // meaningless to all but devs
	LogLevelDeveloperPlus        // even more
	LogLevelAll                  // everything
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
	if lookup == "" {
		return nil, fmt.Errorf("empty uuid looking up client")
	}
	for i := range s.clients {
		if s.clients[i].UUID == lookup {
			return &s.clients[i], nil
		}
	}
	return nil, fmt.Errorf("unknown client: %q", lookup)
}

// Locate the struct of the server for a particular
// name, get a pointer to it
func (s *Server) FindClientByName(name string) (*client.Client, error) {
	if name == "" {
		return nil, fmt.Errorf("empty name looking up client")
	}
	for i := range s.clients {
		if s.clients[i].Name == name {
			return &s.clients[i], nil
		}
	}
	return nil, fmt.Errorf("unknown client: %q", name)
}

// Get a pointer to a user based on their email
func (s *Server) GetUserByEmail(email string) (*pb.User, error) {
	if email == "" {
		return nil, fmt.Errorf("empty email getting user")
	}
	for _, u := range s.users {
		if u.GetEmail() == email {
			return u, nil
		}
	}
	return &pb.User{}, fmt.Errorf("user not found: %q", email)
}

// ClientsByContext will provide a collection of pointers for clients
// accessible to the context.
//
// Circular: find clients by context to include in that context
func ClientsByContext(ctx *IdentityContext) []*client.Client {
	cls := []*client.Client{}
	if ctx == nil {
		return cls
	}
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
	if ident == "" {
		return list
	}
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
	if cl == nil || !cl.Encrypted {
		return
	}
	newkey := crypto.RandomBytes(crypto.AESBlockLength)
	newIV := crypto.RandomBytes(crypto.AESIVLength)
	blob := append(newkey, newIV...)
	(&cl.MessageOut).WriteByte(SCMDKey)
	(&cl.MessageOut).WriteData(blob)
	SendMessages(cl)
	cl.SymmetricKey = newkey
	cl.InitVector = newIV
}

// ParseClients will build a slice of Client structs based on the files on
// disk. Each client is in its own directory in the client directory. The
// clients directory is specified in the main server config. Only clients
// with a valid "settings.pb" file and not disabled will be loaded.
func (s *Server) ParseClients() ([]client.Client, error) {
	var clients []client.Client
	if s == nil {
		return clients, fmt.Errorf("null receiver")
	}
	// this essentially loops through each file in the directory
	err := filepath.WalkDir(s.config.GetClientDirectory(), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking client directory: %v", err)
		}
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			cl, err := client.LoadSettings(info.Name(), s.config.GetClientDirectory())
			if err != nil {
				return nil
			}
			rules, err := cl.FetchRules()
			if err != nil {
				s.Logf(LogLevelInfo, "error fetching rules for %q: %v\n", cl.Name, err)
			}
			cl.Rules = rules
			cl.Server = s
			if cl.Enabled {
				clients = append(clients, cl)
			}
			cl.Invites = client.InviteBucket{
				Tokens: MaxInviteTokens,
				Max:    MaxInviteTokens,
				Freq:   InviteTokenInterval,
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return clients, nil
}

// Write the clients proto to disk as text-format
func MaterializeClients(outfile string, clients []client.Client) error {
	if outfile == "" {
		return fmt.Errorf("empty output file name")
	}
	if len(clients) == 0 {
		return fmt.Errorf("no clients to write")
	}
	clientspb := []*pb.Client{}
	for _, c := range clients {
		p := c.ToProto()
		clientspb = append(clientspb, p)
	}
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

// SendError is a way of letting the client know there's a problem.
func SendError(cl *client.Client, pl *client.Player, severity int, err string) {
	if cl == nil || err == "" {
		return
	}
	out := &cl.MessageOut
	out.WriteByte(SCMDError)
	if pl == nil {
		out.WriteByte(-1)
	} else {
		out.WriteByte(pl.ClientID)
	}
	out.WriteByte(severity)
	out.WriteString(err)
	SendMessages(cl)
}

// Accept (or deny) a new connection.
// The first message sent should identify the game server (our client) and
// trigger the authentication process. If auth succeeds, connection persists
// in a goroutine from this function.
//
// Auth process:
//  1. Client generates a random nonce, encryptes with server's public key,
//     and sends the challenge over with other info in the greeting.
//  2. Server will decrypt the nonce, calculate an SHA256 hash of the data
//     and send it back to the client along with it's own random nonce. The
//     entire response is encrypted with the client's public key.
//  3. Client will decrypt and compare to what the server sent back. If the
//     hashes match, the server has successfully authenticated to the client
//     and can be trusted. Client will hash the decrypted server nonce and
//     send it back to the server. The server will compare hashes and if they
//     match the client will be trusted by the server.
//
// Called from main loop when a new connection is made
func (s *Server) HandleConnection(c net.Conn) {
	defer c.Close()

	input := make([]byte, NetReadLength)
	count, err := c.Read(input)
	if err != nil {
		srv.Logf(LogLevelDebug, "Client read error: %v\n", err)
		return
	}
	msg := message.NewBuffer(input[:count])
	if msg.Length < 5 {
		srv.Logf(LogLevelDebug, "short read before greeting\n")
		return
	}

	if msg.ReadLong() != ProtocolMagic {
		srv.Logf(LogLevelDebug, "invalid client\n")
		srv.Logf(LogLevelDeveloper, "\n%s", hex.Dump(msg.Data))
		return
	}

	srv.Logf(LogLevelNormal, "serving %s\n", c.RemoteAddr().String())
	if msg.ReadByte() != CMDHello {
		srv.Logf(LogLevelNormal, "bad message type, closing connection")
		srv.Logf(LogLevelDeveloper, "\n%s", hex.Dump(msg.Data))
		return
	}

	greeting, err := ParseGreeting(&msg)
	if err != nil {
		srv.Logf(LogLevelNormal, "%v\n", err)
		return
	}

	clNonce, err := crypto.PrivateDecrypt(srv.privateKey, greeting.challenge)
	if err != nil {
		srv.Logf(LogLevelNormal, "%v\n", err)
		return
	}
	hash, err := crypto.MessageDigest(clNonce)
	if err != nil {
		srv.Logf(LogLevelNormal, "%v\n", err)
		return
	}

	cl, err := srv.FindClient(greeting.uuid)
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

	if greeting.version < versionRequired {
		srv.Logf(LogLevelNormal, "game version < %d required, found %d\n", versionRequired, greeting.version)
		cl.Log.Printf("game version < %d required, found %d\n", versionRequired, greeting.version)
		return
	}

	cl.Port = greeting.port
	cl.Encrypted = greeting.encrypted
	cl.Connection = &c
	cl.Connected = true
	cl.Version = greeting.version
	cl.MaxPlayers = greeting.maxPlayers

	keyFile := path.Join(srv.config.ClientDirectory, cl.Name, "key")

	srv.Logf(LogLevelInfo, "[%s] Loading public key: %s\n", cl.Name, keyFile)
	pubkey, err := crypto.LoadPublicKey(keyFile)
	if err != nil {
		srv.Logf(LogLevelNormal, "error loading public key: %v\n", err)
		return
	}
	cl.PublicKey = pubkey

	cl.Challenge = crypto.RandomBytes(challengeLength)
	blob := append(hash, cl.Challenge...)

	// If client requests encrypted transit, generate session keys and append
	if cl.Encrypted {
		cl.SymmetricKey = crypto.RandomBytes(crypto.AESBlockLength)
		cl.InitVector = crypto.RandomBytes(crypto.AESIVLength)
		blob = append(blob, cl.SymmetricKey...)
		blob = append(blob, cl.InitVector...)
	}

	// Encrypt the whole blob with client's public key so only that client can
	// possibly decrypt it.
	blobCipher, err := crypto.PublicEncrypt(cl.PublicKey, blob)
	if err != nil {
		srv.Logf(LogLevelNormal, "[%s] auth failed: %v\n", cl.Name, err)
		return
	}

	out := &cl.MessageOut
	out.WriteByte(SCMDHelloAck)
	out.WriteShort(len(blobCipher))
	out.WriteData(blobCipher)
	SendMessages(cl)

	// read the client signature
	count, err = c.Read(input)
	if err != nil {
		srv.Logf(LogLevelNormal, "error reading client auth response: %v\n", err)
		return
	}

	msg = message.NewBuffer(input[:count])
	verified, err := s.AuthenticateClient(&msg, cl)
	if err != nil {
		srv.Logf(LogLevelNormal, "%v", err)
		SendError(cl, nil, 500, err.Error())
	}

	if !verified {
		srv.Logf(LogLevelNormal, "[%s] authentication failed\n", cl.Name)
		cl.Log.Println("authentication failed, disconnecting")
		return
	}

	srv.Logf(LogLevelNormal, "[%s] authenticated\n", cl.Name)
	cl.Log.Println("authenticated")

	out.WriteByte(SCMDTrusted)
	SendMessages(cl)
	cl.Trusted = true
	cl.ConnectTime = time.Now().Unix()
	cl.Players = make([]client.Player, cl.MaxPlayers)
	cl.Invites = client.InviteBucket{
		Tokens: MaxInviteTokens,
		Max:    MaxInviteTokens,
		Freq:   30,
	}

	// main connection loop for this client
	// - wait for input
	// - parse any messages received, react as necessary
	// - send any responses
	for {
		input := make([]byte, NetReadLength)
		size, err := c.Read(input)
		if err != nil {
			srv.Logf(LogLevelInfo, "[%s] read error: %v\n", cl.Name, err)
			cl.Log.Println("read error:", err)
			break
		}
		if cl.Encrypted && cl.Trusted {
			srv.Logf(LogLevelDeveloperPlus, "encrypted packet received:\n%s", hex.Dump(input[:size]))
			input, size = crypto.SymmetricDecrypt(cl.SymmetricKey, cl.InitVector, input[:size])
			if size == 0 {
				srv.Logf(LogLevelDeveloperPlus, "unable to decrypt using KEY:\n%s\nIV: %s\n", hex.Dump(cl.SymmetricKey), hex.Dump(cl.InitVector))
				// try again with the previous IV in case this message was sent
				// prior to receiving our last.
				input, size = crypto.SymmetricDecrypt(cl.SymmetricKey, cl.PreviousIV, input[:size])
				if size == 0 {
					srv.Logf(LogLevelDeveloperPlus, "unable to decrypt using KEY:\n%s\nPrevIV: %s\n", hex.Dump(cl.SymmetricKey), hex.Dump(cl.PreviousIV))
					srv.Logf(LogLevelNormal, "[%s] error decrypting packet from frontend, disconnecting.\n", cl.Name)
					cl.Log.Println("error decrypting packet, disconnecting.")
					return
				}
			}
		}
		cl.Message = message.NewBuffer(input[:size])
		ParseMessage(cl)
		SendMessages(cl)
	}
	cl.Connection = nil
	cl.Connected = false
	cl.Trusted = false
}

// Send all messages in the outgoing queue to the client (gameserver). If the
// client requested encrypted transit, encrypt using the session key generated
// during the handshake.
func SendMessages(cl *client.Client) {
	if cl == nil || cl.MessageOut.Size() == 0 {
		return
	}
	cl.Server.(*Server).Logf(LogLevelDeveloperPlus, "Sending to client:\n%s\n", hex.Dump(cl.MessageOut.Data))
	if cl.Trusted && cl.Encrypted {
		cipher := crypto.SymmetricEncrypt(cl.SymmetricKey, cl.InitVector, cl.MessageOut.Data[:cl.MessageOut.Index])
		cl.MessageOut = message.NewBuffer(cipher)
		cl.PreviousIV = cl.InitVector
		cl.InitVector = cipher[:crypto.AESIVLength]
	}
	(*cl.Connection).Write(cl.MessageOut.Data)
	(&cl.MessageOut).Reset()
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
	if format == "" {
		return
	}
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

// context logging for server. Will output the date/time, source file name and
// line number, and any included args. Logging is dependant on verbosity level
// from the config. Newline is included.
func (s *Server) Logln(level int, args ...any) {
	if int(s.config.GetVerboseLevel()) < level {
		return
	}
	_, src, line, ok := runtime.Caller(1) // from parent, not here
	if ok && s.config.GetVerboseLevel() > LogLevelNormal {
		preamble := fmt.Sprintf("%s:%d]", path.Base(src), line)
		log.Println(preamble, args)
		return
	}
	log.Println(args...)
}

// Start the cloud admin server
func Startup(configFile string, foreground bool) {
	if configFile == "" {
		log.Fatalln("no config file specified")
	}
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

	srv.Logf(LogLevelInfo, "%-21s %s\n", "loading users:", srv.config.GetUserFile())
	users, err := api.ReadUsersFromDisk(srv.config.GetUserFile())
	if err != nil {
		log.Println(err)
	} else {
		srv.users = users
	}

	srv.Logf(LogLevelNormal, "loading clients from %q\n", srv.config.ClientDirectory)
	clients, err := srv.ParseClients()
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

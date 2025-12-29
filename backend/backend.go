package backend

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
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/database"
	"github.com/packetflinger/q2admind/frontend"
	"github.com/packetflinger/q2admind/maprotator"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// "This" admin server
type Backend struct {
	users      []*pb.User          // website users
	config     pb.Config           // global config
	frontends  []frontend.Frontend // managed quake 2 servers
	rules      []*pb.Rule          // bans/mutes/etc
	privateKey *rsa.PrivateKey     // private to us
	publicKey  *rsa.PublicKey      // known to clients
	maintCount int                 // total maintenance runs
}

var (
	be Backend // this server
	db database.Database
)

const (
	ProtocolMagic       = 1128346193 // "Q2AC"
	versionRequired     = 715        // git revision number
	challengeLength     = 16         // bytes
	TeleportWidth       = 80         // max chars per line for teleport replies
	StifleMax           = 300        // 5 minutes
	GreetingLength      = 306
	NetReadLength       = 5000
	MaxInviteTokens     = 3
	InviteTokenInterval = 300 // seconds per token added
)

// Commands sent from a frontend to us
const (
	_             = iota
	CMDHello      // frontend connect
	CMDQuit       // frontend disconnect
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

// Commands we send back to a frontend
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

// Locate the struct of the frontend for a particular ID, get a pointer to it
func (b *Backend) FindFrontend(lookup string) (*frontend.Frontend, error) {
	if lookup == "" {
		return nil, fmt.Errorf("empty uuid looking up client")
	}
	for i := range b.frontends {
		if b.frontends[i].UUID == lookup {
			return &b.frontends[i], nil
		}
	}
	return nil, fmt.Errorf("unknown client: %q", lookup)
}

// Locate the struct of the frontend for a particular name, get a pointer to it
func (b *Backend) FindFrontendByName(name string) (*frontend.Frontend, error) {
	if name == "" {
		return nil, fmt.Errorf("empty name looking up client")
	}
	for i := range b.frontends {
		if b.frontends[i].Name == name {
			return &b.frontends[i], nil
		}
	}
	return nil, fmt.Errorf("unknown client: %q", name)
}

// Get a pointer to a user based on their email
func (b *Backend) GetUserByEmail(email string) (*pb.User, error) {
	if email == "" {
		return nil, fmt.Errorf("blank email address")
	}
	for _, u := range b.users {
		if u.GetEmail() == email {
			return u, nil
		}
	}
	return nil, nil
}

// FrontendsByContext will provide a collection of pointers for frontends
// accessible to the context.
//
// Circular: find clients by context to include in that context
func FrontendsByContext(ctx *IdentityContext) []*frontend.Frontend {
	cls := []*frontend.Frontend{}
	if ctx == nil {
		return cls
	}
	for i, cl := range be.frontends {
		if cl.Owner == ctx.user.Email {
			cls = append(cls, &be.frontends[i])
			continue
		}
		for _, key := range cl.APIKeys.GetKey() {
			if key.GetSecret() == ctx.apiKey {
				cls = append(cls, &be.frontends[i])
			}
		}
	}
	return cls
}

// Acquire a slice of client pointers that a particular identity has access to
// (owners and delegates)
func FrontendsByIdentity(ident string) []*frontend.Frontend {
	list := []*frontend.Frontend{}
	if ident == "" {
		return list
	}
	for i, cl := range be.frontends {
		if strings.EqualFold(cl.Owner, ident) {
			list = append(list, &be.frontends[i])
		}
	}
	return list
}

// Change symmetric keys. Generate new key and iv and immediately send them to
// the client. This jumps ahead of the normal send buffer so that all messages
// from this point on can be decrypted on the client.
//
// Called from Pong() every hour or so
func RotateKeys(fe *frontend.Frontend) {
	if fe == nil || !fe.Encrypted {
		return
	}
	newkey := crypto.RandomBytes(crypto.AESBlockLength)
	newIV := crypto.RandomBytes(crypto.AESIVLength)
	blob := append(newkey, newIV...)
	(&fe.MessageOut).WriteByte(SCMDKey)
	(&fe.MessageOut).WriteData(blob)
	SendMessages(fe)
	fe.SymmetricKey = newkey
	fe.InitVector = newIV
}

// ParseFrontends will build a slice of Frontend structs based on the files on
// disk. Each frontend is in its own directory in the client directory. The
// clients directory is specified in the main server config. Only clients with
// a valid "settings.pb" file and not disabled will be loaded.
func (b *Backend) ParseFrontends() ([]frontend.Frontend, error) {
	var frontends []frontend.Frontend
	if b == nil {
		return frontends, fmt.Errorf("null receiver")
	}
	// this essentially loops through each file in the directory
	err := filepath.WalkDir(b.config.GetClientDirectory(), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking client directory: %v", err)
		}
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			// the parent folder itself
			if info.Name() == b.config.GetClientDirectory() {
				return nil
			}
			fe, err := frontend.LoadSettings(info.Name(), b.config.GetClientDirectory())
			if err != nil {
				return err
			}
			fe.Data = &db
			rules, err := fe.FetchRules()
			if err != nil {
				b.Logf(LogLevelInfo, "error fetching rules for %q: %v\n", fe.Name, err)
			}
			fe.Rules = rules
			fe.Server = b
			fe.ID, err = fe.GetDatabaseID()
			if err != nil {
				b.Logln(LogLevelInfo, err)
			}
			fe.Invites = frontend.InviteBucket{
				Tokens: MaxInviteTokens,
				Max:    MaxInviteTokens,
				Freq:   InviteTokenInterval,
			}
			fe.Maplist = maprotator.NewMapRotation("default", []string{
				"q2dm7",
				"tltf",
				"q2rdm2",
				"q2dm8",
			})
			if fe.Enabled {
				frontends = append(frontends, fe)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return frontends, nil
}

// Write the frontends proto to disk as text-format
func MaterializeFrontends(outfile string, frontends []frontend.Frontend) error {
	if outfile == "" {
		return fmt.Errorf("empty output file name")
	}
	if len(frontends) == 0 {
		return fmt.Errorf("no clients to write")
	}
	fespb := []*pb.Frontend{}
	for _, c := range frontends {
		p := c.ToProto()
		fespb = append(fespb, p)
	}
	fes := pb.Frontends{
		Frontend: fespb,
	}
	opt := prototext.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}
	textpb, err := opt.Marshal(&fes)
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
func SendError(fe *frontend.Frontend, pl *frontend.Player, severity int, err string) {
	if fe == nil || err == "" {
		return
	}
	out := &fe.MessageOut
	out.WriteByte(SCMDError)
	if pl == nil {
		out.WriteByte(-1)
	} else {
		out.WriteByte(pl.ClientID)
	}
	out.WriteByte(severity)
	out.WriteString(err)
	SendMessages(fe)
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
func (b *Backend) HandleConnection(c net.Conn) {
	defer c.Close()

	input := make([]byte, NetReadLength)
	count, err := c.Read(input)
	if err != nil {
		be.Logf(LogLevelDebug, "Frontend read error: %v\n", err)
		return
	}
	msg := message.NewBuffer(input[:count])
	if msg.Length < 5 {
		be.Logf(LogLevelDebug, "short read before greeting\n")
		return
	}

	if msg.ReadLong() != ProtocolMagic {
		be.Logf(LogLevelDebug, "invalid frontend\n")
		be.Logf(LogLevelDeveloper, "\n%s", hex.Dump(msg.Data))
		return
	}

	be.Logf(LogLevelNormal, "serving %s\n", c.RemoteAddr().String())
	if msg.ReadByte() != CMDHello {
		be.Logf(LogLevelNormal, "bad message type, closing connection")
		be.Logf(LogLevelDeveloper, "\n%s", hex.Dump(msg.Data))
		return
	}

	greeting, err := ParseGreeting(&msg)
	if err != nil {
		be.Logf(LogLevelNormal, "%v\n", err)
		return
	}

	clNonce, err := crypto.PrivateDecrypt(be.privateKey, greeting.challenge)
	if err != nil {
		be.Logf(LogLevelNormal, "%v\n", err)
		return
	}
	hash, err := crypto.MessageDigest(clNonce)
	if err != nil {
		be.Logf(LogLevelNormal, "%v\n", err)
		return
	}

	fe, err := be.FindFrontend(greeting.uuid)
	if err != nil {
		log.Println(err)
		return
	}

	fe.Path = path.Join(be.config.GetClientDirectory(), fe.Name)

	fe.Log, err = NewFrontendLogger(fe)
	if err != nil {
		be.Logf(LogLevelNormal, "[%s] error creating logger: %v\n", fe.Name, err)
	}
	fe.Log.Printf("[%s] connecting...\n", fe.IPAddress)

	if greeting.version < versionRequired {
		be.Logf(LogLevelNormal, "game version < %d required, found %d\n", versionRequired, greeting.version)
		fe.Log.Printf("game version < %d required, found %d\n", versionRequired, greeting.version)
		return
	}

	fe.Port = greeting.port
	fe.Encrypted = greeting.encrypted
	fe.Connection = &c
	fe.Connected = true
	fe.Version = greeting.version
	fe.MaxPlayers = greeting.maxPlayers

	keyFile := path.Join(be.config.ClientDirectory, fe.Name, "key")

	be.Logf(LogLevelInfo, "[%s] Loading public key: %s\n", fe.Name, keyFile)
	pubkey, err := crypto.LoadPublicKey(keyFile)
	if err != nil {
		be.Logf(LogLevelNormal, "error loading public key: %v\n", err)
		return
	}
	fe.PublicKey = pubkey

	fe.Challenge = crypto.RandomBytes(challengeLength)
	blob := append(hash, fe.Challenge...)

	// If client requests encrypted transit, generate session keys and append
	if fe.Encrypted {
		fe.SymmetricKey = crypto.RandomBytes(crypto.AESBlockLength)
		fe.InitVector = crypto.RandomBytes(crypto.AESIVLength)
		blob = append(blob, fe.SymmetricKey...)
		blob = append(blob, fe.InitVector...)
	}

	// Encrypt the whole blob with client's public key so only that client can
	// possibly decrypt it.
	blobCipher, err := crypto.PublicEncrypt(fe.PublicKey, blob)
	if err != nil {
		be.Logf(LogLevelNormal, "[%s] auth failed: %v\n", fe.Name, err)
		return
	}

	out := &fe.MessageOut
	out.WriteByte(SCMDHelloAck)
	out.WriteShort(len(blobCipher))
	out.WriteData(blobCipher)
	SendMessages(fe)

	// read the client signature
	count, err = c.Read(input)
	if err != nil {
		be.Logf(LogLevelNormal, "error reading client auth response: %v\n", err)
		return
	}

	msg = message.NewBuffer(input[:count])
	verified, err := b.AuthenticateClient(&msg, fe)
	if err != nil {
		be.Logf(LogLevelNormal, "%v", err)
		SendError(fe, nil, 500, err.Error())
	}

	if !verified {
		be.Logf(LogLevelNormal, "[%s] authentication failed\n", fe.Name)
		fe.Log.Println("authentication failed, disconnecting")
		return
	}

	be.Logf(LogLevelNormal, "[%s] authenticated\n", fe.Name)
	fe.Log.Println("authenticated")

	out.WriteByte(SCMDTrusted)
	SendMessages(fe)
	fe.Trusted = true
	fe.ConnectTime = time.Now().Unix()
	fe.Players = make([]frontend.Player, fe.MaxPlayers)
	fe.Invites = frontend.InviteBucket{
		Tokens: MaxInviteTokens,
		Max:    MaxInviteTokens,
		Freq:   30,
	}

	vars, err := fe.FetchServerVars()
	if err != nil {
		be.Logf(LogLevelInfo, "error fetching %q vars: %v", fe.Name, err)
	}
	fe.ServerVars = vars

	// main connection loop for this frontend
	// - wait for input
	// - parse any messages received, react as necessary
	// - send any responses
	for {
		input := make([]byte, NetReadLength)
		size, err := c.Read(input)
		if err != nil {
			be.Logf(LogLevelInfo, "[%s] read error: %v\n", fe.Name, err)
			fe.Log.Println("read error:", err)
			break
		}
		if fe.Encrypted && fe.Trusted {
			be.Logf(LogLevelDeveloperPlus, "encrypted packet received:\n%s", hex.Dump(input[:size]))
			input, size = crypto.SymmetricDecrypt(fe.SymmetricKey, fe.InitVector, input[:size])
			if size == 0 {
				be.Logf(LogLevelDeveloperPlus, "unable to decrypt using KEY:\n%s\nIV: %s\n", hex.Dump(fe.SymmetricKey), hex.Dump(fe.InitVector))
				// try again with the previous IV in case this message was sent
				// prior to receiving our last.
				input, size = crypto.SymmetricDecrypt(fe.SymmetricKey, fe.PreviousIV, input[:size])
				if size == 0 {
					be.Logf(LogLevelDeveloperPlus, "unable to decrypt using KEY:\n%s\nPrevIV: %s\n", hex.Dump(fe.SymmetricKey), hex.Dump(fe.PreviousIV))
					be.Logf(LogLevelNormal, "[%s] error decrypting packet from frontend, disconnecting.\n", fe.Name)
					fe.Log.Println("error decrypting packet, disconnecting.")
					return
				}
			}
		}
		fe.Message = message.NewBuffer(input[:size])
		ParseMessage(fe)
		SendMessages(fe)
	}
	err = fe.Seen()
	if err != nil {
		be.Logln(LogLevelInfo, err)
	}
	fe.Connection = nil
	fe.Connected = false
	fe.Trusted = false
}

// Send all messages in the outgoing queue to the frontend. If the frontend
// requested encrypted transit, encrypt using the session key generated during
// the handshake.
func SendMessages(fe *frontend.Frontend) {
	if fe == nil || fe.MessageOut.Size() == 0 {
		return
	}
	fe.Server.(*Backend).Logf(LogLevelDeveloperPlus, "Sending to client:\n%s\n", hex.Dump(fe.MessageOut.Data))
	if fe.Trusted && fe.Encrypted {
		cipher := crypto.SymmetricEncrypt(fe.SymmetricKey, fe.InitVector, fe.MessageOut.Data[:fe.MessageOut.Index])
		fe.MessageOut = message.NewBuffer(cipher)
		fe.PreviousIV = fe.InitVector
		fe.InitVector = cipher[:crypto.AESIVLength]
	}
	(*fe.Connection).Write(fe.MessageOut.Data)
	(&fe.MessageOut).Reset()
}

// Get list of frontends this email address has access to
func (b *Backend) UserFrontends(u string) []*frontend.Frontend {
	var out []*frontend.Frontend
	if u == "" {
		return out
	}
	for _, f := range b.frontends {
		for k := range f.WebUsers {
			if strings.EqualFold(k, u) {
				out = append(out, &f)
			}
		}
	}
	return out
}

// Gracefully shut everything down. Close database connection, write states to
// disk, etc
func Shutdown() {
	be.Logf(LogLevelNormal, "Shutting down...")
	db.Handle.Close() // not sure if this is necessary
}

// Context logging for server. Will output the date/time, source file name and
// line number, and a formatted string. Logging is dependant on verbosity level
// from the config.
func (b *Backend) Logf(level int, format string, args ...any) {
	if format == "" {
		return
	}
	if int(b.config.GetVerboseLevel()) < level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	_, src, line, ok := runtime.Caller(1) // from parent, not here
	if ok && b.config.GetVerboseLevel() > LogLevelNormal {
		log.Printf("%s:%d] %s", path.Base(src), line, msg)
		return
	}
	log.Print(msg)
}

// context logging for backend. Will output the date/time, source file name and
// line number, and any included args. Logging is dependant on verbosity level
// from the config. Newline is included.
func (b *Backend) Logln(level int, args ...any) {
	if int(b.config.GetVerboseLevel()) < level {
		return
	}
	_, src, line, ok := runtime.Caller(1) // from parent, not here
	if ok && b.config.GetVerboseLevel() > LogLevelNormal {
		preamble := fmt.Sprintf("%s:%d]", path.Base(src), line)
		log.Println(preamble, args)
		return
	}
	log.Println(args...)
}

// Start the cloud admin server backend
func Startup(configFile string, foreground bool) {
	if configFile == "" {
		log.Fatalln("no config file specified")
	}
	log.Printf("%-21s %s\n", "Loading config:", configFile)
	textpb, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = prototext.Unmarshal(textpb, &be.config)
	if err != nil {
		log.Fatal(err)
	}

	if !foreground {
		f, err := os.OpenFile(be.config.GetLogFile(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	be.Logf(LogLevelInfo, "%-21s %s\n", "loading private key:", be.config.GetPrivateKey())
	privkey, err := crypto.LoadPrivateKey(be.config.GetPrivateKey())
	if err != nil {
		log.Fatalf("error loading private key: %v\n", err)
	}

	pubkey := privkey.Public().(*rsa.PublicKey)
	be.privateKey = privkey
	be.publicKey = pubkey

	be.Logf(LogLevelInfo, "%-21s %s\n", "opening database:", be.config.Database)
	db, err = database.Open(be.config.Database)
	if err != nil {
		log.Fatalln(err)
	}

	be.Logf(LogLevelInfo, "%-21s %s\n", "loading global rules:", be.config.GetRuleFile())
	rules, err := FetchRules(be.config.GetRuleFile())
	if err != nil {
		log.Fatal(err)
	} else {
		be.rules = rules
	}

	be.Logf(LogLevelInfo, "%-21s %s\n", "loading users:", be.config.GetUserFile())
	users, err := api.ReadUsersFromDisk(be.config.GetUserFile())
	if err != nil {
		log.Fatal(err)
	} else {
		be.users = users
	}
	be.Logf(LogLevelInfo, "  found %d users\n", len(be.users))

	be.Logf(LogLevelNormal, "loading clients from %q\n", be.config.ClientDirectory)
	frontends, err := be.ParseFrontends()
	if err != nil {
		log.Fatal(err)
	} else {
		slices.SortFunc(frontends, func(a, b frontend.Frontend) int {
			return strings.Compare(a.Name, b.Name)
		})
		be.frontends = frontends
		for _, c := range be.frontends {
			be.Logf(LogLevelNormal, "  %-25s [%s:%d]", c.Name, c.IPAddress, c.Port)
		}
	}

	port := fmt.Sprintf("%s:%d", be.config.Address, be.config.Port)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		log.Fatalln(err)
	}
	defer listener.Close()

	be.Logf(LogLevelNormal, "listening for gameservers on %s\n", port)

	if be.config.GetApiEnabled() {
		creds, err := ReadOAuthCredsFromDisk(be.config.GetAuthFile())
		if err != nil {
			log.Println(err)
		}
		secret := []byte(be.config.GetApiSecret())
		if len(secret) < 16 {
			secret = crypto.RandomBytes(16)
		}
		go be.RunHTTPServer(be.config.GetApiAddress(), int(be.config.GetApiPort()), creds, secret)
	}

	go be.startMaintenance()
	go be.startManagement()
	go be.startSSHServer()

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Fatalln(err)
		}
		go be.HandleConnection(c)
	}
}

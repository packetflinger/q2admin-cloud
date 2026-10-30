package backend

import (
	"context"
	"crypto/rsa"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize/english"
	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/database"
	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

type Backend struct {
	config      *pb.Config           // global config
	frontends   []*frontend.Frontend // managed quake 2 servers
	frontendsMu sync.Mutex           // frontend slice lock
	maintCount  int                  // total maintenance runs
	privateKey  *rsa.PrivateKey      // private to us
	publicKey   *rsa.PublicKey       // known to clients
	rpcCancel   context.CancelFunc   // call this to kill the RPC server
	rpcStart    int64                // unix timestamp
	rules       []*pb.Rule           // bans/mutes/etc
	users       []*pb.User           // website users
}

var (
	be Backend // this server
	db database.Database
)

const (
	challengeLength     = 16         // bytes
	versionRequired     = 1306       // q2admin git revision number
	GreetingLength      = 306        // for spot checking validity
	InviteTokenInterval = 300        // seconds per token added
	MaxInviteTokens     = 3          // for invite token bucket
	ProtocolMagic       = 1128346193 // "Q2AC"
	StifleMax           = 300        // 5 minutes
	TeleportWidth       = 80         // max chars per line for teleport replies
)

// Framing. Everything sent between a frontend and us, in both directions and
// from the very first byte, is a series of frames: a 4-byte little-endian
// length, then that many payload bytes. The payload is one or more whole
// commands. Once the connection is trusted and encryption is on, the payload
// is those commands sealed with AES-128-GCM (see crypto.Stream): the
// ciphertext followed by a 16-byte tag. The length is never encrypted and
// counts the ciphertext and tag. Each frame is sealed on its own and a
// command never spans frames. An empty frame is allowed and ignored.
//
// TCP is a stream, so without the length a single Read() can return part of
// a message or several at once. This has to match q2admin's g_cloud.h
// (CLOUD_FRAME_HEADER).
const (
	FrameHeaderLength = 4
	FrameMaxLength    = 0x55FF - FrameHeaderLength // q2admin's QUEUE_SIZE less the header
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
	CMDKeyAck // the frontend switched to the key from our last SCMDKey
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

// apiSecret returns the key that signs session cookies. If the config has
// none (or one too short), a random one is generated and saved, so sessions
// survive restarts.
func apiSecret(c *pb.Config) ([]byte, error) {
	if len(c.GetApiSecret()) >= 16 {
		return []byte(c.GetApiSecret()), nil
	}
	c.ApiSecret = hex.EncodeToString(crypto.RandomBytes(32))
	if _, err := db.Handle.Exec("UPDATE config SET api_secret = ?", c.ApiSecret); err != nil {
		return nil, fmt.Errorf("error saving the generated api secret: %v", err)
	}
	return []byte(c.ApiSecret), nil
}

// Fetch the config from the database and return it in proto form
func (b *Backend) LoadConfig() (*pb.Config, error) {
	var c pb.Config
	qry := "SELECT * FROM config"
	r := db.Handle.QueryRow(qry)
	err := r.Scan(&c.DebugMode, &c.VerboseLevel, &c.Address, &c.Port, &c.PrivateKey, &c.ApiEnabled, &c.ApiAddress, &c.ApiPort, &c.WebRoot, &c.MaintenanceTime, &c.RpcAddress, &c.RpcPort, &c.LogFile, &c.SshAddress, &c.SshPort, &c.SshHostkey, &c.AuthFile, &c.ApiSecret)
	if err != nil {
		return nil, fmt.Errorf("error scanning config from database: %v", err)
	}
	return &c, nil
}

// Fetch the config from the database and return it in proto form
func (b *Backend) LoadUsers() ([]*pb.User, error) {
	var users []*pb.User
	qry := "SELECT * FROM user"
	r, err := db.Handle.Query(qry)
	if err != nil {
		return users, fmt.Errorf("error selecting users: %v", err)
	}
	defer r.Close()
	var nullID int
	for r.Next() {
		var u pb.User
		// name is added last by database.Open
		err := r.Scan(&nullID, &u.Uuid, &u.Email, &u.Disabled, &u.AllowSsh, &u.PublicKey, &u.AllowRpc, &u.RpcPublicKey, &u.Name)
		if err != nil {
			return users, fmt.Errorf("error scanning users from database: %v", err)
		}
		users = append(users, &u)
	}
	return users, nil
}

// Load the private key from it's PEM-encoded form to an actual struct. The
// associated public key struct is also returned at the same time.
func (b *Backend) LoadKeys(keydata []byte) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privkey, err := crypto.LoadPrivateKey(keydata)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading private key: %v\n", err)
	}
	pubkey := privkey.Public().(*rsa.PublicKey)
	return privkey, pubkey, nil
}

// CreateFrontend will add a new record for a frontend in the database and in
// memory.
func (b *Backend) CreateFrontend(f *frontend.Frontend) error {
	qry := `
	INSERT INTO frontend 
		(uuid, name, owner, enabled, description, allow_teleport, allow_invite, ip_address, port, public_key_data, verified)
	VALUES
		(?,?,?,?,?,?,?,?,?,?,?)`
	res, err := db.Handle.Exec(qry, f.UUID, f.Name, f.Owner, f.Enabled, f.Description, f.AllowTeleport, f.AllowInvite, f.IPAddress, f.Port, f.PublicKeyData, f.Verified)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	f.ID = int(id)
	f.Data = &db
	b.frontendsMu.Lock()
	b.frontends = append(b.frontends, f)
	b.frontendsMu.Unlock()
	return nil
}

// DeleteFrontend will remove a particular frontend from the database and the
// backend's list of known frontends. If the frontend is connected at the time
// it'll immediately be disconnected.
func (b *Backend) DeleteFrontend(name string) error {
	fe, err := b.FindFrontendByName(name)
	if err != nil {
		return fmt.Errorf("error finding frontend %q: %v", name, err)
	}
	qry := "DELETE FROM frontend WHERE id = ?"
	_, err = db.Handle.Exec(qry, fe.ID)
	if err != nil {
		return fmt.Errorf("error removing frontend %q from database: %v", name, err)
	}
	if _, err = db.Handle.Exec("DELETE FROM server_group_member WHERE frontend_id = ?", fe.ID); err != nil {
		return fmt.Errorf("error removing frontend %q from its groups: %v", name, err)
	}
	if _, err = db.Handle.Exec("DELETE FROM frontend_delegate WHERE frontend_id = ?", fe.ID); err != nil {
		return fmt.Errorf("error removing frontend %q's delegates: %v", name, err)
	}
	// a new slice, so snapshots from Frontends() stay intact
	b.frontendsMu.Lock()
	defer b.frontendsMu.Unlock()
	b.frontends = slices.DeleteFunc(slices.Clone(b.frontends), func(f *frontend.Frontend) bool { return f == fe })
	return nil
}

// Locate the struct of the frontend for a particular ID, get a pointer to it
func (b *Backend) FindFrontend(lookup string) (*frontend.Frontend, error) {
	if lookup == "" {
		return nil, fmt.Errorf("empty uuid looking up client")
	}
	b.frontendsMu.Lock()
	defer b.frontendsMu.Unlock()
	for i := range b.frontends {
		if b.frontends[i].UUID == lookup {
			return b.frontends[i], nil
		}
	}
	return nil, fmt.Errorf("unknown client: %q", lookup)
}

// Locate the struct of the frontend for a particular name, get a pointer to it
func (b *Backend) FindFrontendByName(name string) (*frontend.Frontend, error) {
	if name == "" {
		return nil, fmt.Errorf("empty name looking up client")
	}
	b.frontendsMu.Lock()
	defer b.frontendsMu.Unlock()
	for i := range b.frontends {
		if b.frontends[i].Name == name {
			return b.frontends[i], nil
		}
	}
	return nil, fmt.Errorf("unknown client: %q", name)
}

// Frontends returns a snapshot of the managed frontends that's safe to range
// over while frontends are added or deleted. Only appends and fresh slices
// are ever stored, so a snapshot's elements never change underneath it.
func (b *Backend) Frontends() []*frontend.Frontend {
	b.frontendsMu.Lock()
	defer b.frontendsMu.Unlock()
	return b.frontends
}

// Get a pointer to a user based on their email
func (b *Backend) GetUserByEmail(email string) (*pb.User, error) {
	if email == "" {
		return nil, fmt.Errorf("blank email address")
	}
	for _, u := range b.users {
		if strings.EqualFold(u.GetEmail(), email) {
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
	for _, cl := range be.Frontends() {
		if cl.Owner == ctx.user.Email {
			cls = append(cls, cl)
			continue
		}
		for _, key := range cl.APIKeys.GetKey() {
			if key.GetSecret() == ctx.apiKey {
				cls = append(cls, cl)
			}
		}
	}
	return cls
}

// FrontendsByIdentity returns the frontends the user with this email owns or
// has been delegated, in Frontends() order. It locks each frontend briefly, so
// don't call it holding one.
func FrontendsByIdentity(ident string) []*frontend.Frontend {
	list := []*frontend.Frontend{}
	if ident == "" {
		return list
	}
	for _, cl := range be.Frontends() {
		cl.Mu.Lock()
		ok := cl.CanAccess(ident)
		cl.Mu.Unlock()
		if ok {
			list = append(list, cl)
		}
	}
	return list
}

// RotateKeys replaces the AES session key. The new key goes to the frontend
// in an SCMDKey, sealed with the old one, and everything we send after that
// frame uses the new key. The frontend keeps using the old key for what it
// sends until it has read the SCMDKey; it then sends CMDKeyAck in its last
// frame under the old key, and we switch our receiving side when we parse
// that (see ParseKeyAck). The same handover TLS 1.3 uses for KeyUpdate.
// Frames can be in flight both ways while this happens, so each direction
// switches at a point both ends agree on, rather than both at once.
//
// Called from Pong() every hour or so, on the connection's goroutine with
// fe.Mu held.
func RotateKeys(fe *frontend.Frontend) {
	if fe == nil || !fe.Encrypted || !fe.Trusted {
		return
	}
	// still waiting for the frontend to ack the last one
	if fe.NextRecv != nil {
		return
	}
	newkey := crypto.RandomBytes(crypto.AESKeyLength)
	send, err := crypto.NewStream(newkey, crypto.ToFrontend)
	if err != nil {
		be.Logf(LogLevelNormal, "[%s] key rotation: %v\n", fe.Name, err)
		return
	}
	recv, err := crypto.NewStream(newkey, crypto.ToBackend)
	if err != nil {
		be.Logf(LogLevelNormal, "[%s] key rotation: %v\n", fe.Name, err)
		return
	}
	(&fe.MessageOut).WriteByte(SCMDKey)
	(&fe.MessageOut).WriteData(newkey)
	if sendMessages(fe, send) {
		fe.NextRecv = recv
	}
}

// ParseKeyAck handles CMDKeyAck: the frontend has switched to the key from
// our last SCMDKey, and every frame it sends after this one is sealed with
// it. This frame was opened with the old key, so the switch takes effect from
// the next frame.
//
// Returns false if no rotation was waiting for an ack, which means the two
// ends disagree about the keys and the connection can't continue.
func ParseKeyAck(fe *frontend.Frontend) bool {
	if fe == nil || fe.NextRecv == nil {
		return false
	}
	fe.RecvStream = fe.NextRecv
	fe.NextRecv = nil
	return true
}

// Fetch all frontends from the database
func (b *Backend) LoadFrontends() ([]*frontend.Frontend, error) {
	var fes []*frontend.Frontend
	qry := "SELECT * FROM frontend"
	rs, err := db.Handle.Query(qry)
	if err != nil {
		return nil, fmt.Errorf("error selecting frontends: %v", err)
	}
	defer rs.Close()
	for rs.Next() {
		fe := &frontend.Frontend{}
		err = rs.Scan(&fe.ID, &fe.UUID, &fe.Name, &fe.Owner, &fe.Enabled, &fe.Description, &fe.AllowTeleport, &fe.AllowInvite, &fe.IPAddress, &fe.Port, &fe.PublicKeyData, &fe.Verified, &fe.Invites.Tokens, &fe.Invites.Freq, &fe.DeleteProtect)
		if err != nil {
			return fes, fmt.Errorf("error scanning frontend: %v", err)
		}
		fe.Data = &db
		fe.Rules, err = fe.FetchRules()
		if err != nil {
			b.Logln(LogLevelInfo, err)
			fe.Rules = nil
		}
		fes = append(fes, fe)
	}
	if err := rs.Err(); err != nil {
		return fes, err
	}
	delegates, err := db.Handle.Query("SELECT frontend_id, email FROM frontend_delegate ORDER BY email")
	if err != nil {
		return fes, fmt.Errorf("error selecting delegates: %v", err)
	}
	defer delegates.Close()
	for delegates.Next() {
		var id int
		var email string
		if err := delegates.Scan(&id, &email); err != nil {
			return fes, fmt.Errorf("error scanning delegates: %v", err)
		}
		for _, fe := range fes {
			if fe.ID == id {
				fe.Delegates = append(fe.Delegates, email)
			}
		}
	}
	slices.SortFunc(fes, func(a, b *frontend.Frontend) int {
		return strings.Compare(a.Name, b.Name)
	})
	return fes, delegates.Err()
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

	input, err := ReadFrame(c)
	if err != nil {
		be.Logf(LogLevelDebug, "Frontend read error: %v\n", err)
		return
	}
	msg := message.NewBuffer(input)
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

	fe.Path = path.Join("clients", fe.Name)

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
	fe.Server = &be

	pubkey, err := crypto.LoadPublicKey([]byte(fe.PublicKeyData))
	if err != nil {
		be.Logf(LogLevelNormal, "error loading public key: %v\n", err)
		return
	}
	fe.PublicKey = pubkey
	fe.Challenge = crypto.RandomBytes(challengeLength)
	blob := append(hash, fe.Challenge...)

	// If client requests encrypted transit, generate the session key and
	// append it. Nothing else is needed: GCM nonces come from counting frames
	// (see crypto.Stream), so both ends start at zero.
	if fe.Encrypted {
		key := crypto.RandomBytes(crypto.AESKeyLength)
		send, err := crypto.NewStream(key, crypto.ToFrontend)
		if err != nil {
			be.Logf(LogLevelNormal, "[%s] session key: %v\n", fe.Name, err)
			return
		}
		recv, err := crypto.NewStream(key, crypto.ToBackend)
		if err != nil {
			be.Logf(LogLevelNormal, "[%s] session key: %v\n", fe.Name, err)
			return
		}
		fe.SendMu.Lock()
		fe.SendStream = send
		fe.SendMu.Unlock()
		fe.RecvStream = recv
		fe.NextRecv = nil
		blob = append(blob, key...)
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
	input, err = ReadFrame(c)
	if err != nil {
		be.Logf(LogLevelNormal, "error reading client auth response: %v\n", err)
		return
	}

	msg = message.NewBuffer(input)
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
		payload, err := ReadFrame(c)
		if err != nil {
			if errors.Is(err, io.EOF) {
				be.Logf(LogLevelInfo, "[%s] disconnected\n", fe.Name)
				fe.Log.Println("frontend disconnected")
				break
			}
			be.Logf(LogLevelInfo, "[%s] read error: %v\n", fe.Name, err)
			fe.Log.Println("read error:", err)
			break
		}
		if len(payload) == 0 {
			continue
		}
		input := payload
		if fe.Encrypted && fe.Trusted {
			be.Logf(LogLevelDeveloperPlus, "encrypted packet received:\n%s", hex.Dump(payload))
			input, err = fe.RecvStream.Open(payload)
			if err != nil {
				// The frame was altered, or the two ends' keys or frame
				// counts no longer match. Either way nothing after it can
				// be opened either.
				be.Logf(LogLevelNormal, "[%s] %v, disconnecting.\n", fe.Name, err)
				fe.Log.Println("bad encrypted frame, disconnecting:", err)
				break
			}
		}
		fe.Mu.Lock()
		fe.Message = message.NewBuffer(input)
		ParseMessage(fe)
		SendMessages(fe)
		fe.Mu.Unlock()
	}
	err = fe.Seen()
	if err != nil {
		be.Logln(LogLevelInfo, err)
	}
	fe.Mu.Lock()
	fe.SendMu.Lock()
	fe.Connection = nil
	fe.SendStream = nil
	fe.SendMu.Unlock()
	fe.RecvStream = nil
	fe.NextRecv = nil
	fe.Connected = false
	fe.Trusted = false
	fe.Mu.Unlock()
}

// ReadFrame reads one whole frame from r (see FrameHeaderLength) and returns
// its payload, waiting for as many reads as it takes to arrive. A length over
// FrameMaxLength is an error: there's no way to find the next frame after it,
// so the connection can't continue. Returns io.EOF if r closes cleanly
// between frames, io.ErrUnexpectedEOF if it closes partway through one.
func ReadFrame(r io.Reader) ([]byte, error) {
	var header [FrameHeaderLength]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(header[:])
	if length > FrameMaxLength {
		return nil, fmt.Errorf("frame too long (%d bytes)", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return payload, nil
}

// MakeFrame puts the 4-byte little-endian length in front of payload, ready
// to send.
func MakeFrame(payload []byte) []byte {
	frame := make([]byte, FrameHeaderLength+len(payload))
	binary.LittleEndian.PutUint32(frame, uint32(len(payload)))
	copy(frame[FrameHeaderLength:], payload)
	return frame
}

// Send all messages in the outgoing queue to the frontend as one frame. If the
// frontend requested encrypted transit, seal it with the session key (see
// crypto.Stream). A frame too large for the frontend to receive is dropped
// (and logged) rather than sent: q2admin would have to drop the connection,
// since it couldn't find the next frame.
func SendMessages(fe *frontend.Frontend) {
	sendMessages(fe, nil)
}

// sendMessages sends the outgoing queue as one frame, like SendMessages. If
// next isn't nil, frames after this one are sealed with it instead (a key
// rotation). Returns whether the frame was written.
//
// fe.SendMu is held from sealing to writing. Frames are sent from several
// goroutines, and the frontend counts frames to know each one's nonce, so
// they have to reach the wire in the order they were sealed.
func sendMessages(fe *frontend.Frontend, next *crypto.Stream) bool {
	if fe == nil || fe.MessageOut.Size() == 0 {
		return false
	}
	fe.SendMu.Lock()
	defer fe.SendMu.Unlock()
	if fe.Connection == nil {
		(&fe.MessageOut).Reset()
		return false
	}
	srv := fe.Server.(*Backend)
	srv.Logf(LogLevelDeveloperPlus, "Sending to client:\n%s\n", hex.Dump(fe.MessageOut.Data))
	payload := fe.MessageOut.Data
	encrypt := fe.Trusted && fe.Encrypted
	size := len(payload)
	if encrypt {
		size += crypto.GCMTagLength
	}
	// checked before sealing, so a dropped frame doesn't use up a nonce
	if size > FrameMaxLength {
		(&fe.MessageOut).Reset()
		srv.Logf(LogLevelNormal, "[%s] outgoing frame too large (%d bytes), dropped\n", fe.Name, size)
		return false
	}
	if encrypt {
		sealed, err := fe.SendStream.Seal(payload)
		if err != nil {
			(&fe.MessageOut).Reset()
			srv.Logf(LogLevelNormal, "[%s] %v, disconnecting\n", fe.Name, err)
			(*fe.Connection).Close()
			return false
		}
		payload = sealed
	}
	(&fe.MessageOut).Reset()
	if _, err := (*fe.Connection).Write(MakeFrame(payload)); err != nil {
		srv.Logf(LogLevelDebug, "[%s] write error: %v\n", fe.Name, err)
		return false
	}
	if next != nil {
		fe.SendStream = next
	}
	return true
}

// Gracefully shut everything down. Close database connection, write states to
// disk, etc
func Shutdown() {
	be.Logf(LogLevelNormal, "Shutdown initiated...")
	be.Logf(LogLevelNormal, "  Writing all player stats to database")
	var err error
	for _, f := range be.Frontends() {
		err = f.WritePlayers()
		if err != nil {
			be.Logf(LogLevelNormal, "    error writing players for %q: %v", f.Name, err)
		}
	}
	be.Logf(LogLevelNormal, "  Closing database")
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
func Startup(databaseFile string, foreground bool) {
	var err error
	db, err = database.Open(databaseFile)
	if err != nil {
		log.Fatalln(err)
	}
	be.config, err = be.LoadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	if !foreground {
		f, err := os.OpenFile(be.config.GetLogFile(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			log.Fatalln(err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	be.privateKey, be.publicKey, err = be.LoadKeys([]byte(be.config.PrivateKey))
	if err != nil {
		log.Fatalf("error loading backend private key: %v\n", err)
	}
	/*
		be.Logf(LogLevelInfo, "%-21s %s\n", "loading global rules:", be.config.GetRuleFile())
		rules, err := FetchRules(be.config.GetRuleFile())
		if err != nil {
			log.Fatal(err)
		} else {
			be.rules = rules
		}
	*/

	be.users, err = be.LoadUsers()
	if err != nil {
		be.Logf(LogLevelInfo, "%s\n", err)
	}
	be.Logf(LogLevelInfo, "Loaded %s\n", english.Plural(len(be.users), "user", ""))

	be.frontends, err = be.LoadFrontends()
	if err != nil {
		log.Fatal(err)
	}
	be.Logf(LogLevelNormal, "loaded %s:\n", english.Plural(len(be.frontends), "frontend", ""))
	for _, c := range be.frontends {
		be.Logf(LogLevelNormal, "  %-25s [%s:%d]", c.Name, c.IPAddress, c.Port)
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
		secret, err := apiSecret(be.config)
		if err != nil {
			log.Fatalln(err)
		}
		go be.RunHTTPServer(be.config.GetApiAddress(), int(be.config.GetApiPort()), creds, secret)
	}

	go be.startMaintenance()
	go be.startRPCServer()
	go be.startSSHServer()

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Fatalln(err)
		}
		go be.HandleConnection(c)
	}
}

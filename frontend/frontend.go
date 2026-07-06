// In this system, a frontend is a Quake 2 game server. They are servers to
// their connected players, but clients to us.
package frontend

import (
	"crypto/rsa"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/libq2/state"
	"github.com/packetflinger/q2admind/database"
	"github.com/packetflinger/q2admind/maprotator"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// This struct is partially populated by parsing a config file
// on disk during init and the rest is filled in when the game
// server actually connects
type Frontend struct {
	AllowInvite   bool                    // honor invites from players
	AllowTeleport bool                    // enable teleport functionality
	APIKeys       *pb.ApiKeys             // keys generated for accessing this client
	Challenge     []byte                  // random data for auth set by server
	Connected     bool                    // is it currently connected to us?
	Connection    *net.Conn               // the tcp connection
	ConnectTime   int64                   // unix timestamp when connection made
	CurrentMap    string                  // what map is currently running
	Data          *database.Database      // pointer to database
	DeleteProtect bool                    // can be deleted or not
	Description   string                  // used in teleporting
	Enabled       bool                    // actually use it
	Encrypted     bool                    // are the messages AES encrypted?
	ID            int                     // this is the database index, remove later
	InitVector    []byte                  // AES IV,
	Invites       InviteBucket            // Invite throttling
	IPAddress     string                  // used for teleporting
	Log           *log.Logger             // log stuff here
	LogFile       *os.File                // pointer to file so we can close when client disconnects
	Maplist       *maprotator.MapList     // the maps for the frontend
	MaxPlayers    int                     // total number
	Message       message.Buffer          // incoming byte stream
	MessageOut    message.Buffer          // outgoing byte stream
	Name          string                  // the teleport name
	Owner         string                  // email addr
	Path          string                  // the fs path for this client
	PingCount     int                     // how many pings client has seen
	PlayerCount   int                     // len(Players)
	Players       []Player                // all the connected players
	Port          int                     // used for teleporting
	PreviousIV    []byte                  // the next to last AES IV we used (just in case)
	PreviousMap   string                  // what was the last map?
	PublicKey     *rsa.PublicKey          // supplied by owner via website
	PublicKeyData string                  // the contents of the `key` file
	Rules         []*pb.Rule              // bans, mutes, etc
	Server        any                     // pointer for circular reference back
	ServerVars    map[string]string       // public server cvars
	SymmetricKey  []byte                  // AES 128 CBC
	TeleportCount int                     // how many times teleport was used
	Terminals     []*chan string          // pointers to the console streams
	Trusted       bool                    // signature challenge verified
	Users         map[*pb.User][]*pb.Role // users who have access via ssh/web
	UUID          string                  // random identifier
	Verified      bool                    // client owner proved they're the owner
	Version       int                     // q2admin library version
	WebUsers      map[string]bool         // key is email addr, val is write access
}

// Each frontend has a small collection of invite tokens available. As players
// use the invite command, tokens are removed. The command won't work once the
// token count reaches 0. The bucket is refilled by the maintenance thread one
// token at a time at a specific interval (every 5 minutes).
//
// Why? Because invites have a high likelihood of being abused. Either players
// spamming them or cross-server shit-talking via invite is expected. Players
// also each have their own token bucket for invites.
type InviteBucket struct {
	Tokens       int
	Max          int
	Freq         int64 // how often to add one (in seconds)
	LastAddition int64 // unix timestamp
	UseCount     int   // how many times invite has been used
}

// Add a token to a particular invite bucket. A token will only be added if
// it's appropriate: below max level and it's been long enough since the last
// addition.
func (b *InviteBucket) InviteBucketAdd() {
	if b == nil {
		return
	}
	now := time.Now().Unix()
	if b.Tokens < b.Max && now-b.LastAddition > b.Freq {
		log.Printf("Adding invite token to bucket (%d/%d)\n", b.Tokens, b.Max)
		b.Tokens++
		b.LastAddition = now
	}
}

// Read rules from disk and return a scoped slice of them
func (fe *Frontend) FetchRules() ([]*pb.Rule, error) {
	var rules []*pb.Rule
	if fe == nil {
		return rules, fmt.Errorf("error fetching rules: null receiver")
	}
	filename := path.Join(fe.Path, "rules.pb")
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	rl := pb.Rules{}
	err = prototext.Unmarshal(contents, &rl)
	if err != nil {
		return nil, err
	}
	rules = rl.GetRule()
	fe.ScopeRules("client", rules)
	return rules, nil
}

// ScopeRules will add a token to the `Scope` property all the rules in a set.
// This is used to mark rules in different contexts (server-level vs client-
// level)
func (fe *Frontend) ScopeRules(scope string, rules []*pb.Rule) {
	if fe == nil || scope == "" {
		return
	}
	for i := range rules {
		rules[i].Scope = scope
	}
}

// MaterializeRules will write the current list of rules to disk. Frontend
// rules are always found in the <frontend>/rules.pb file.
func (fe *Frontend) MaterializeRules(rules []*pb.Rule) error {
	if fe == nil {
		return fmt.Errorf("error writing rules: null receiver")
	}
	if len(rules) == 0 {
		return nil
	}
	collection := &pb.Rules{Rule: rules}
	filename := path.Join(fe.Path, "rules.pb")
	data, err := prototext.MarshalOptions{Indent: "  "}.Marshal(collection)
	if err != nil {
		return fmt.Errorf("error marshalling rules: %v", err)
	}
	header := []byte("# proto-file: proto/serverconfig.proto\n# proto-message: Rules\n\n")
	data = append(header, data...)
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing rules to %q: %v", filename, err)
	}
	return nil
}

// Read settings file for client from disk and make a Frontend struct
// from them.
func LoadSettings(name string, clientsDir string) (Frontend, error) {
	var fe Frontend
	if name == "" {
		return fe, fmt.Errorf("error loading settings: blank client name supplied")
	}
	if clientsDir == "" {
		return fe, fmt.Errorf("error loading settings: blank client directly supplied")
	}
	filename := path.Join(clientsDir, name, "settings.pb")
	contents, err := os.ReadFile(filename)
	if err != nil {
		return fe, err
	}
	fes := pb.Frontends{}
	err = prototext.Unmarshal(contents, &fes)
	if err != nil {
		return fe, err
	}

	for _, f := range fes.GetFrontend() {
		if f.GetName() != name {
			continue
		}
		fe.Name = f.GetName()
		fe.Owner = f.GetOwner()
		fe.Description = f.GetDescription()
		fe.UUID = f.GetUuid()
		fe.Path = path.Join(clientsDir, fe.Name)
		fe.Enabled = !f.GetDisabled()
		fe.AllowInvite = f.GetAllowInvite()
		fe.AllowTeleport = f.GetAllowTeleport()

		tokens := strings.Split(f.GetAddress(), ":")
		if len(tokens) == 2 {
			fe.Port, err = strconv.Atoi(tokens[1])
			if err != nil {
				fe.Port = 27910
			}
		} else {
			fe.Port = 27910
		}
		fe.IPAddress = tokens[0]
		fe.WebUsers = make(map[string]bool)
		for _, user := range f.GetUsers() {
			fe.WebUsers[user.GetEmail()] = (user.GetAccess() == "write")
		}
		contents, err := os.ReadFile(path.Join(clientsDir, name, "key"))
		if err == nil { // is nil!
			fe.PublicKeyData = string(contents)
		}
		rules, err := fe.FetchRules()
		if err != nil {
			log.Printf("error fetching rules for %q: %v\n", fe.Name, err)
		}
		fe.Rules = rules
	}
	return fe, nil
}

// GetPlayerFromPrint attempts to identify a player object associated with a
// print message. Returns a slice since Q2 allows multiple players to use the
// same name.
//
// Quake 2 servers don't care who said what, they just send an
// svc_print in the format of "<name>: message". Team-based messages have the
// name in parens. The only delimiter is ": " between the name and message.
// But ": " is allowed in both player names and the messages they type.
//
// Examples:
//
//	"claire: nice shot!" - valid
//	"best: me: Nice Shot!!" - valid (player name "best: me")
//	"worst: you: hahah, nice shot: dumbass" - valid (player name "worst: you")
//
// So, if there is only 1 ": " in the string, it's easy. If more than one, loop
// through the known players on the map and try to
func (fe *Frontend) GetPlayerFromPrint(txt string) ([]*Player, error) {
	var players []*Player
	var name string
	if fe == nil {
		return players, fmt.Errorf("can't resolve player: nil receiver")
	}
	if txt == "" {
		return players, fmt.Errorf("can't resolve player: blank input")
	}
	count := strings.Count(txt, ": ") // note the space
	if count == 0 {
		return nil, errors.New("no name in print")
	} else {
		tokens := strings.Split(txt, ": ")
		if len(tokens) > 1 {
			name = tokens[0]
		}
		for i, p := range fe.Players {
			if p.Name == name {
				players = append(players, &fe.Players[i])
			}
		}
	}
	return players, nil
}

// ResolvePlayers will find and return pointers to players matching the input
// text. This can be a client number or a (partial) name string. Input is
// lightly sanitized because it's user-supplied.
//
// If the input is a number, assume it's the client ID and not the name. IDs
// are the only way to uniquely identify players, names can overlap.
func (fe *Frontend) ResolvePlayers(in string) ([]*Player, error) {
	var found []*Player
	if in == "" {
		return []*Player{}, fmt.Errorf("empty player input")
	}
	if len(in) > 15 {
		in = in[:14]
	}
	id, err := strconv.Atoi(in)
	if id < 0 || id > fe.MaxPlayers {
		return []*Player{}, fmt.Errorf("invalid player ID")
	}
	if err != nil {
		// it's not a number
		for _, p := range fe.Players {
			if strings.Contains(strings.ToLower(p.Name), strings.ToLower(in)) {
				found = append(found, &p)
			}
		}
		return found, nil
	}
	// it's a number
	for _, p := range fe.Players {
		if p.ClientID == id && p.ConnectTime > 0 {
			found = append(found, &p)
		}
	}
	return found, nil
}

// ToProto will convert a Frontend struct into the corresponding protobuf. This
// is used when materializing the frontends to disk.
func (fe *Frontend) ToProto() *pb.Frontend {
	if fe == nil {
		return &pb.Frontend{}
	}
	var users []*pb.FrontendUser
	for k, v := range fe.WebUsers {
		access := "read"
		if v {
			access = "write"
		}
		users = append(users, &pb.FrontendUser{Email: k, Access: access})
	}
	return &pb.Frontend{
		Address:       fmt.Sprintf("%s:%d", fe.IPAddress, fe.Port),
		Name:          fe.Name,
		Uuid:          fe.UUID,
		Description:   fe.Description,
		Owner:         fe.Owner,
		Verified:      fe.Verified,
		AllowTeleport: fe.AllowTeleport,
		AllowInvite:   fe.AllowInvite,
		Users:         users,
	}
}

// Find all players that match the name provided. Multiple players
// are allowed to have the same name at the same time, this will
// return all of them.
func (fe *Frontend) PlayersByName(name string) ([]*Player, error) {
	var players []*Player
	if fe == nil {
		return players, fmt.Errorf("error getting players by name: null receiver")
	}
	if name == "" {
		return players, errors.New("blank name argument")
	}
	for i, p := range fe.Players {
		if p.Name == name {
			players = append(players, &fe.Players[i])
		}
	}
	return players, nil
}

// SSHPrintln will send the value of text to all the SSH-connected clients.
func (fe *Frontend) SSHPrintln(text string) {
	if fe == nil || text == "" {
		return
	}
	for i := range fe.Terminals {
		select {
		case *fe.Terminals[i] <- text:
			// log.Printf("Sending %q to ssh client %d\n", text, i)
		default:
			// log.Println("doing nothing")
		}
	}
}

// The terminal goroutine will call this when disconnecting so the frontend can
// close the console stream channel.
func (fe *Frontend) TerminalDisconnected(t *chan string) []*chan string {
	var terms []*chan string
	if fe == nil {
		return terms
	}
	for i := range fe.Terminals {
		if fe.Terminals[i] == t {
			close(*fe.Terminals[i])
			continue
		}
		terms = append(terms, fe.Terminals[i])
	}
	return terms
}

// Query the frontend for all the server vars
func (fe *Frontend) FetchServerVars() (map[string]string, error) {
	s := &state.Server{Address: fe.IPAddress, Port: fe.Port}
	vars, err := s.FetchInfo()
	return vars.Server, err
}

// AddPlayer will insert the player into the database
func (fe *Frontend) AddPlayer(pl *Player) error {
	if pl == nil {
		return fmt.Errorf("error adding player to db: null player")
	}
	qry := `
		INSERT INTO player (server, name, ip, hostname, vpn, cookie, version, userinfo, time) 
		VALUES (?,?,?,?,?,?,?,?,?)`
	res, err := fe.Data.Handle.Exec(
		qry, pl.Frontend.Name, pl.Name, pl.IP, pl.Hostname, pl.VPN,
		pl.Cookie, pl.Version, pl.Userinfo, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("error inserting player %s[%s]: %v", pl.Name, pl.IP, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("error getting id from inserted player %q: %v", pl.Name, err)
	}
	pl.Database_ID = id
	return nil
}

// Get the ID used in the database of a particular frontend. If this is a new
// frontend, ensure it's setup correctly in the database whether it's an old
// existing one or brand new.
func (fe *Frontend) GetDatabaseID() (int, error) {
	var id int
	qry := "SELECT id FROM frontend WHERE uuid = ? LIMIT 1"
	err := fe.Data.Handle.QueryRow(qry, fe.UUID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			qry = "INSERT INTO frontend (uuid) VALUES (?)"
			res, err := fe.Data.Handle.Exec(qry, fe.UUID)
			if err != nil {
				return -1, fmt.Errorf("error inserting frontend %q in database: %v", fe.UUID, err)
			}
			liid, err := res.LastInsertId()
			if err != nil {
				return -1, fmt.Errorf("error getting frontend id for %q in database: %v", fe.UUID, err)
			}
			id = int(liid)

			qry = "INSERT INTO connection (frontend, last_seen) VALUES (?,?)"
			res, err = fe.Data.Handle.Exec(qry, id, time.Now().Unix())
			if err != nil {
				return -1, fmt.Errorf("error inserting frontend last seen %q in database: %v", fe.UUID, err)
			}
		} else {
			return -1, fmt.Errorf("error setting up frontend %q in database: %v", fe.UUID, err)
		}
	}
	return id, nil
}

// Record when this frontend was last seen in the database
func (fe *Frontend) Seen() error {
	if fe.Data == nil {
		return fmt.Errorf("null database pointer in %q, can't update seen data", fe.Name)
	}
	qry := "UPDATE connection SET last_seen = ? WHERE frontend = ?"
	_, err := fe.Data.Handle.Exec(qry, time.Now().Unix(), fe.ID)
	if err != nil {
		return fmt.Errorf("error writing last_seen to database: %v", err)
	}
	return nil
}

func (fe *Frontend) GetLastSeen() int64 {
	var seen int64
	qry := "SELECT last_seen FROM connection WHERE frontend = ? LIMIT 1"
	err := fe.Data.Handle.QueryRow(qry, fe.ID).Scan(seen)
	if errors.Is(err, sql.ErrNoRows) {
		return -1
	}
	return seen
}

// Materialize will write the current frontend to disk as a textproto.
func (fe *Frontend) Materialize() error {
	if fe == nil {
		return fmt.Errorf("error writing rules: null receiver")
	}
	p := &pb.Frontends{
		Frontend: []*pb.Frontend{fe.ToProto()},
	}
	filename := path.Join(fe.Path, "settings.pb")
	data, err := prototext.MarshalOptions{Indent: "  "}.Marshal(p)
	if err != nil {
		return fmt.Errorf("error marshalling rules: %v", err)
	}
	header := []byte("# proto-file: proto/frontend.proto\n# proto-message: Frontends\n\n")
	data = append(header, data...)
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing rules to %q: %v", filename, err)
	}
	return nil
}

// Get the roles associated with a user for this frontend
func (fe *Frontend) FetchUserRoles(u *pb.User) ([]*pb.Role, error) {
	if u == nil {
		return nil, fmt.Errorf("unknown user")
	}
	return fe.Users[u], nil
}

// Calculate the current kill:death ratio for a player based on their current
// stats. Special consideration needs to be made for cases were no deaths
// have been reported (divide by zero).
//
// When a player has more deaths than frags, the absolute value of the
// numerator and denominator are swapped and multiplied by -1 resulting in a
// larger negative number instead of an increasingly smaller number as deaths
// increase.
func (fe *Frontend) CalculateKDR(cid int) float64 {
	if cid < 0 || cid >= fe.MaxPlayers {
		return 0.0
	}
	p, err := fe.FindPlayer(cid)
	if err != nil {
		fmt.Println(err)
		return 0.0
	}
	nom, denom := p.Frags, p.Deaths
	multiplier := 1
	if denom > nom {
		nom, denom = denom, nom
		multiplier = -1
	}
	if denom == 0 {
		return math.Abs(float64(nom)) * float64(multiplier)
	}
	return math.Abs(float64(nom)/float64(denom)) * float64(multiplier)
}

// WritePlayer will write a player's stats to the database. This includes data
// like their frag counts, death counts, KDR, etc. This will typically happen
// when the player quits (or teleports) or in the event the backend server is
// shutdown.
func (fe *Frontend) WritePlayer(client int) error {
	pls := fe.Players
	if client < 0 || client >= fe.MaxPlayers {
		return fmt.Errorf("invalid client id: %d", client)
	}
	if pls[client].ConnectTime == 0 {
		return nil
	}
	p := pls[client]
	qry := `
			INSERT INTO player_stat
				(player, frags, deaths, suicides, kdr, play_time)
			VALUES
				(?,?,?,?,?,?)`
	_, err := fe.Data.Handle.Exec(qry,
		p.Database_ID,
		p.Frags,
		p.Deaths,
		p.Suicides,
		p.KDR,
		(time.Now().Unix() - p.ConnectTime),
	)
	if err != nil {
		return fmt.Errorf("error writing player %s:%d[%d]: %v", fe.Name, p.ClientID, p.Database_ID, err)
	}
	return nil
}

// Write player data to the database for all players on a particular frontend.
func (fe *Frontend) WritePlayers() error {
	var err error
	for _, p := range fe.Players {
		err = fe.WritePlayer(p.ClientID)
		if err != nil {
			return err
		}
	}
	return nil
}

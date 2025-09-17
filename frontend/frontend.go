// In this system, a frontend is a Quake 2 game server. They are servers to
// their connected players, but clients to us.
package frontend

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/libq2/state"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// This struct is partially populated by parsing a config file
// on disk during init and the rest is filled in when the game
// server actually connects
type Frontend struct {
	ID            int                     // this is the database index, remove later
	UUID          string                  // random identifier
	Owner         string                  // email addr
	Version       int                     // q2admin library version
	Name          string                  // the teleport name
	Description   string                  // used in teleporting
	IPAddress     string                  // used for teleporting
	Port          int                     // used for teleporting
	Connected     bool                    // is it currently connected to us?
	Verified      bool                    // client owner proved they're the owner
	CurrentMap    string                  // what map is currently running
	PreviousMap   string                  // what was the last map?
	Enabled       bool                    // actually use it
	Connection    *net.Conn               // the tcp connection
	Players       []Player                // all the connected players
	PlayerCount   int                     // len(Players)
	MaxPlayers    int                     // total number
	Message       message.Buffer          // incoming byte stream
	MessageOut    message.Buffer          // outgoing byte stream
	Encrypted     bool                    // are the messages AES encrypted?
	Trusted       bool                    // signature challenge verified
	PublicKey     *rsa.PublicKey          // supplied by owner via website
	SymmetricKey  []byte                  // AES 128 CBC
	InitVector    []byte                  // AES IV,
	PreviousIV    []byte                  // the next to last AES IV we used (just in case)
	Rules         []*pb.Rule              // bans, mutes, etc
	PingCount     int                     // how many pings client has seen
	Log           *log.Logger             // log stuff here
	LogFile       *os.File                // pointer to file so we can close when client disconnects
	APIKeys       *pb.ApiKeys             // keys generated for accessing this client
	Path          string                  // the fs path for this client
	Terminals     []*chan string          // pointers to the console streams
	Users         map[*pb.User][]*pb.Role // users who have access via ssh/web
	Challenge     []byte                  // random data for auth set by server
	ConnectTime   int64                   // unix timestamp when connection made
	Server        any                     // pointer for circular reference back
	AllowInvite   bool                    // honor invites from players
	Invites       InviteBucket            // Invite throttling
	AllowTeleport bool                    // enable teleport functionality
	TeleportCount int                     // how many times teleport was used
	ServerVars    map[string]string       // public server cvars
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
		return players, fmt.Errorf("error getting player from print: null receiver")
	}
	if txt == "" {
		return players, fmt.Errorf("error getting player from print: empty print")
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

// ToProto will convert a Frontend struct into the corresponding protobuf. This
// is used when materializing the frontends to disk.
func (fe *Frontend) ToProto() *pb.Frontend {
	if fe == nil {
		return &pb.Frontend{}
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

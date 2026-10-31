// In this system, a frontend is a Quake 2 game server. They are servers to
// their connected players, but clients to us.
package frontend

import (
	"cmp"
	"crypto/rsa"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/libq2/state"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/database"
	"github.com/packetflinger/q2admind/maprotator"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// terminalsMu guards every Frontend's Terminals and console backlog. It's
// package-level because Frontend values are copied around and can't hold a
// mutex.
var terminalsMu sync.Mutex

// ConsoleBacklog is how many recent console lines each frontend keeps, so a
// console opened later (web or SSH) starts with recent history.
const ConsoleBacklog = 500

// ConsoleLine is one line of a frontend's console.
type ConsoleLine struct {
	Seq  uint64 `json:"-"` // counts up per frontend from 1, for resuming a stream
	Time int64  `json:"t"` // unix seconds
	Text string `json:"m"`
}

// This struct is partially populated by parsing a config file
// on disk during init and the rest is filled in when the game
// server actually connects
type Frontend struct {
	AllowInvite   bool                // honor invites from players
	AllowTeleport bool                // enable teleport functionality
	APIKeys       *pb.ApiKeys         // keys generated for accessing this client
	Challenge     []byte              // random data for auth set by server
	Connected     bool                // is it currently connected to us?
	Connection    *net.Conn           // the tcp connection
	ConnectTime   int64               // unix timestamp when connection made
	CurrentMap    string              // what map is currently running
	Data          *database.Database  // pointer to database
	DeleteProtect bool                // can be deleted or not
	Description   string              // used in teleporting
	Enabled       bool                // actually use it
	Encrypted     bool                // are the messages AES encrypted?
	ID            int                 // this is the database index, remove later
	Invites       InviteBucket        // Invite throttling
	Kills         map[string]int      // kills by weapon since the last stats sample
	IPAddress     string              // used for teleporting
	Log           *log.Logger         // log stuff here
	LogFile       *os.File            // pointer to file so we can close when client disconnects
	Maplist       *maprotator.MapList // the maps for the frontend
	MaxPlayers    int                 // total number
	Message       message.Buffer      // incoming byte stream
	MessageOut    message.Buffer      // outgoing byte stream
	Mu            sync.Mutex          // guards state shared by the connection goroutine, SSH sessions and maintenance
	Name          string              // the teleport name
	NextRecv      *crypto.Stream      // after a key rotation, replaces RecvStream once q2admin acks it
	Owner         string              // email addr
	Delegates     []string            // emails of users the owner shared this frontend with
	Path          string              // the fs path for this client
	PingCount     int                 // how many pings client has seen
	PlayerCount   int                 // len(Players)
	Players       []Player            // all the connected players
	Port          int                 // used for teleporting
	PreviousMap   string              // what was the last map?
	PublicKey     *rsa.PublicKey      // supplied by owner via website
	PublicKeyData string              // the contents of the `key` file
	RecvStream    *crypto.Stream      // decrypts frames from q2admin (when Encrypted)
	Rules         []*pb.Rule          // bans, mutes, etc
	Server        any                 // pointer for circular reference back
	SendMu        sync.Mutex          // serializes sends, so each frame gets the next nonce in wire order
	SendStream    *crypto.Stream      // encrypts frames to q2admin (when Encrypted)
	ServerVars    map[string]string   // public server cvars
	TeleportCount int                 // how many times teleport was used
	Terminals     []*chan ConsoleLine // pointers to the console streams
	console       []ConsoleLine       // the last ConsoleBacklog lines, oldest first
	consoleSeq    uint64              // Seq of the newest console line
	Trusted       bool                // signature challenge verified
	UUID          string              // random identifier
	Verified      bool                // client owner proved they're the owner
	Version       int                 // q2admin library version
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

// ruleColumns are the rule table columns the rule queries read and write, in
// order. A rule belongs to a frontend or to a server group, so one of frontend
// and server_group is 0. List fields are stored one entry per line; userinfo
// as "property=value" lines.
const ruleColumns = `frontend, uuid, rule_type, creation_time, expiration_time, delay,
	stifle_length, disabled, scope, timespec_before, timespec_after,
	timespec_every, timespec_playtime, addresses, names, hostnames, clients,
	userinfo, message, description, vpn, server_group`

// RuleSet is where a set of rules is stored: one frontend's (Frontend set) or
// one server group's (Group set).
type RuleSet struct {
	DB       *database.Database
	Frontend int
	Group    int64
}

// rules is fe's own rule set.
func (fe *Frontend) rules() RuleSet { return RuleSet{DB: fe.Data, Frontend: fe.ID} }

// FetchRules loads this frontend's own rules from the database.
func (fe *Frontend) FetchRules() ([]*pb.Rule, error) { return fe.rules().Fetch() }

// FetchGroupRules loads the rules of every server group fe is in. They're read
// fresh each time, so joining or leaving a group takes effect straight away.
func (fe *Frontend) FetchGroupRules() ([]*pb.Rule, error) {
	return queryRules(fe.Data, "server_group IN (SELECT group_id FROM server_group_member WHERE frontend_id = ?)", fe.ID)
}

// InsertRule saves a new rule for this frontend, filling in its database id.
func (fe *Frontend) InsertRule(r *pb.Rule) error { return fe.rules().Insert(r) }

// UpdateRule saves changes to one of this frontend's rules, matched by UUID.
func (fe *Frontend) UpdateRule(r *pb.Rule) error { return fe.rules().Update(r) }

// DeleteRule removes one of this frontend's rules from the database.
func (fe *Frontend) DeleteRule(uuid string) error { return fe.rules().Delete(uuid) }

// owner is the WHERE clause and argument matching this set's rules.
func (s RuleSet) owner() (string, []any) {
	return "frontend = ? AND server_group = ?", []any{s.Frontend, s.Group}
}

// Fetch loads the set's rules.
func (s RuleSet) Fetch() ([]*pb.Rule, error) {
	where, args := s.owner()
	return queryRules(s.DB, where, args...)
}

// queryRules loads the rules matching a WHERE clause.
func queryRules(d *database.Database, where string, args ...any) ([]*pb.Rule, error) {
	var rules []*pb.Rule
	qry := `SELECT id, frontend, COALESCE(uuid, ''), rule_type, COALESCE(creation_time, 0),
		COALESCE(expiration_time, 0), COALESCE(delay, 0), COALESCE(stifle_length, 0),
		COALESCE(disabled, 0), COALESCE(scope, ''), COALESCE(timespec_before, ''),
		COALESCE(timespec_after, ''), COALESCE(timespec_every, ''),
		COALESCE(timespec_playtime, ''), COALESCE(addresses, ''), COALESCE(names, ''),
		COALESCE(hostnames, ''), COALESCE(clients, ''), COALESCE(userinfo, ''),
		COALESCE(message, ''), COALESCE(description, ''), COALESCE(vpn, 0)
		FROM rule WHERE ` + where
	rows, err := d.Handle.Query(qry, args...)
	if err != nil {
		return rules, fmt.Errorf("error selecting rules: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		r := &pb.Rule{Timespec: &pb.TimeSpec{}}
		var addresses, names, hostnames, clients, userinfo, message, description string
		err := rows.Scan(&r.Id, &r.FrontendId, &r.Uuid, &r.Type, &r.CreationTime, &r.ExpirationTime,
			&r.Delay, &r.StifleLength, &r.Disabled, &r.Scope, &r.Timespec.Before, &r.Timespec.After,
			&r.Timespec.Every, &r.Timespec.PlayTime, &addresses, &names, &hostnames, &clients,
			&userinfo, &message, &description, &r.Vpn)
		if err != nil {
			return rules, fmt.Errorf("error scanning rules: %v", err)
		}
		r.Address = splitLines(addresses)
		r.Name = splitLines(names)
		r.Hostname = splitLines(hostnames)
		r.Client = splitLines(clients)
		r.Message = splitLines(message)
		r.Description = splitLines(description)
		for _, line := range splitLines(userinfo) {
			k, v, _ := strings.Cut(line, "=")
			r.UserInfo = append(r.UserInfo, &pb.UserInfo{Property: k, Value: v})
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// splitLines is the inverse of strings.Join(list, "\n") that drops blanks.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// values returns r's values in ruleColumns order, owned by this set.
func (s RuleSet) values(r *pb.Rule) []any {
	ts := r.GetTimespec()
	var userinfo []string
	for _, ui := range r.GetUserInfo() {
		userinfo = append(userinfo, ui.GetProperty()+"="+ui.GetValue())
	}
	join := func(l []string) string { return strings.Join(l, "\n") }
	return []any{s.Frontend, r.GetUuid(), r.GetType(), r.GetCreationTime(), r.GetExpirationTime(),
		r.GetDelay(), r.GetStifleLength(), r.GetDisabled(), r.GetScope(), ts.GetBefore(),
		ts.GetAfter(), ts.GetEvery(), ts.GetPlayTime(), join(r.GetAddress()), join(r.GetName()),
		join(r.GetHostname()), join(r.GetClient()), join(userinfo), join(r.GetMessage()),
		join(r.GetDescription()), r.GetVpn(), s.Group}
}

// Insert saves a new rule in the set, filling in its database id.
func (s RuleSet) Insert(r *pb.Rule) error {
	if r.CreationTime == 0 {
		r.CreationTime = time.Now().Unix()
	}
	cols := strings.Count(ruleColumns, ",") + 1
	qry := "INSERT INTO rule (" + ruleColumns + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", cols), ",") + ")"
	res, err := s.DB.Handle.Exec(qry, s.values(r)...)
	if err != nil {
		return fmt.Errorf("error inserting rule: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("error getting new rule id: %v", err)
	}
	r.Id = int32(id)
	r.FrontendId = int32(s.Frontend)
	return nil
}

// Update saves changes to one of the set's rules, matched by UUID.
func (s RuleSet) Update(r *pb.Rule) error {
	cols := strings.Fields(strings.ReplaceAll(ruleColumns, ",", " "))
	where, args := s.owner()
	qry := "UPDATE rule SET " + strings.Join(cols, " = ?, ") + " = ? WHERE uuid = ? AND " + where
	if _, err := s.DB.Handle.Exec(qry, append(append(s.values(r), r.GetUuid()), args...)...); err != nil {
		return fmt.Errorf("error updating rule %q: %v", r.GetUuid(), err)
	}
	return nil
}

// Delete removes one of the set's rules.
func (s RuleSet) Delete(uuid string) error {
	where, args := s.owner()
	if _, err := s.DB.Handle.Exec("DELETE FROM rule WHERE uuid = ? AND "+where, append([]any{uuid}, args...)...); err != nil {
		return fmt.Errorf("error deleting rule %q: %v", uuid, err)
	}
	return nil
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
	if id < 0 || id >= fe.MaxPlayers {
		return []*Player{}, fmt.Errorf("invalid player ID")
	}
	if err != nil {
		// it's not a number
		for i, p := range fe.Players {
			if p.ConnectTime > 0 && strings.Contains(strings.ToLower(p.Name), strings.ToLower(in)) {
				found = append(found, &fe.Players[i])
			}
		}
		return found, nil
	}
	// it's a number
	for i, p := range fe.Players {
		if p.ClientID == id && p.ConnectTime > 0 {
			found = append(found, &fe.Players[i])
		}
	}
	return found, nil
}

// CanAccess reports whether the user with this email owns fe or has had it
// delegated to them. Call it holding fe.Mu.
func (fe *Frontend) CanAccess(email string) bool {
	if email == "" {
		return false
	}
	return strings.EqualFold(fe.Owner, email) || slices.ContainsFunc(fe.Delegates, func(d string) bool {
		return strings.EqualFold(d, email)
	})
}

// ToProto will convert a Frontend struct into the corresponding protobuf. This
// is used when materializing the frontends to disk.
func (fe *Frontend) ToProto() *pb.Frontend {
	if fe == nil {
		return &pb.Frontend{}
	}
	var users []*pb.FrontendUser
	for _, email := range fe.Delegates {
		users = append(users, &pb.FrontendUser{Email: email, Access: "delegate"})
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

// ConsoleBroadcast sends text to everything watching this frontend's console,
// SSH sessions and web console streams, and keeps it in the backlog.
func (fe *Frontend) ConsoleBroadcast(text string) {
	if fe == nil || text == "" {
		return
	}
	terminalsMu.Lock()
	defer terminalsMu.Unlock()
	fe.consoleSeq++
	line := ConsoleLine{Seq: fe.consoleSeq, Time: time.Now().Unix(), Text: text}
	fe.console = append(fe.console, line)
	if len(fe.console) > ConsoleBacklog {
		fe.console = slices.Clone(fe.console[len(fe.console)-ConsoleBacklog:])
	}
	for i := range fe.Terminals {
		select {
		case *fe.Terminals[i] <- line:
		default: // a watcher that's this far behind misses the line
		}
	}
}

// AddTerminal subscribes t to this frontend's console stream and returns the
// backlog lines after sequence number after (0 for all of it), to show before
// the live lines. Subscribing and reading the backlog happen together, so no
// line is missed or repeated between them.
func (fe *Frontend) AddTerminal(t *chan ConsoleLine, after uint64) []ConsoleLine {
	terminalsMu.Lock()
	defer terminalsMu.Unlock()
	fe.Terminals = append(fe.Terminals, t)
	if after > fe.consoleSeq { // from before a restart, when numbering began again
		after = 0
	}
	i, _ := slices.BinarySearchFunc(fe.console, after+1, func(l ConsoleLine, seq uint64) int {
		return cmp.Compare(l.Seq, seq)
	})
	return slices.Clone(fe.console[i:])
}

// The terminal goroutine will call this when disconnecting to unsubscribe from
// the console stream. The channel isn't closed, nothing ranges over it.
func (fe *Frontend) TerminalDisconnected(t *chan ConsoleLine) {
	if fe == nil {
		return
	}
	terminalsMu.Lock()
	defer terminalsMu.Unlock()
	fe.Terminals = slices.DeleteFunc(fe.Terminals, func(c *chan ConsoleLine) bool { return c == t })
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
	return KDR(p.Frags, p.Deaths)
}

// KDR is the kill:death ratio for these totals, negative when deaths
// outnumber frags (see CalculateKDR).
func KDR(frags, deaths int) float64 {
	nom, denom := frags, deaths
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

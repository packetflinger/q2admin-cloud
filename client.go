// In this system, a "client" is a Quake 2 game server.
// They are servers to their connected players, but
// clients to us.
package main

import (
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

// This struct is partially populated by parsing disk a file
// on disk on init and the rest is filled in when the game
// server actually connects
type Client struct {
	ID          int               // this is the database index, remove later
	UUID        string            // random identifier
	Owner       string            // email addr
	Version     int               // what version are we running
	Name        string            // the teleport name
	Description string            // used in teleporting
	IPAddress   string            // used for teleporting
	Port        int               // used for teleporting
	Connected   bool              // is it currently connected to us?
	Verified    bool              // client owner proved they're the owner
	CurrentMap  string            // what map is currently running
	Enabled     bool              // actually use it
	Connection  *net.Conn         // the tcp connection
	Players     []Player          // all the connected players
	PlayerCount int               // len(Players)
	MaxPlayers  int               // total number
	Message     MessageBuffer     // incoming byte stream
	MessageOut  MessageBuffer     // outgoing byte stream
	Encrypted   bool              // are the messages AES encrypted?
	Trusted     bool              // signature challenge verified
	PublicKey   *rsa.PublicKey    // supplied by owner via website
	AESKey      []byte            // 16 (128bit)
	AESIV       []byte            // 16 bytes (CBC)
	Rules       []ClientRule      // bans, mutes, etc
	PingCount   int               // how many pings client has seen
	WebSockets  []*websocket.Conn // slice of web clients
}

// JSON structure for persistent storage
type ClientDiskFormat struct {
	UUID          string `json:"UUID"` // match client to server config
	AllowTeleport bool   `json:"AllowTeleport"`
	AllowInvite   bool   `json:"AllowInvite"`
	Enabled       bool   `json:"Enabled"`
	Verified      bool   `json:"Verified"`
	Address       string `json:"Address"`
	Name          string `json:"Name"`        // teleport name, must be unique
	Owner         string `json:"Owner"`       // ID from UserFormat
	Description   string `json:"Description"` // shows up in teleport
	Contacts      string `json:"Contacts"`    // for getting ahold of operator
	/*PublicKey     string             `json:"PublicKey"`   // relative path to file */
	Rules []ClientRuleFormat `json:"Controls"`
}

// Locate the struct of the server for a particular
// ID, get a pointer to it
func FindClient(lookup string) (*Client, error) {
	for i, cl := range q2a.clients {
		if cl.UUID == lookup {
			return &q2a.clients[i], nil
		}
	}

	return nil, errors.New("unknown client")
}

// The file should be just a list of server names one per line
// comments (// and #) and blank lines are allowed
// indenting doesn't matter
//
// Called from initialize() at startup
func (s RemoteAdminServer) ReadClientFile() []string {
	contents, err := os.ReadFile(s.config.GetClientFile())
	if err != nil {
		log.Println(err)
		os.Exit(0)
	}

	srvs := []string{}
	lines := strings.Split(string(contents), "\n")
	for i := range lines {
		trimmed := strings.Trim(lines[i], " \t")
		// remove empty lines
		if trimmed == "" {
			continue
		}
		// remove comments
		if trimmed[0] == '#' || trimmed[0:2] == "//" {
			continue
		}
		srvs = append(srvs, trimmed)
	}
	return srvs
}

// Send all messages in the outgoing queue to the client (gameserver)
func (cl *Client) SendMessages() {
	if !cl.Connected {
		return
	}

	//fmt.Printf("%s", hex.Dump(cl.MessageOut.buffer[:cl.MessageOut.length]))

	// keys have been exchanged, encrypt the message
	if cl.Trusted && cl.Encrypted {
		cipher := SymmetricEncrypt(
			cl.AESKey,
			cl.AESIV,
			cl.MessageOut.buffer[:cl.MessageOut.length])

		clearmsg(&cl.MessageOut)
		cl.MessageOut.buffer = cipher
		cl.MessageOut.length = len(cipher)
	}

	// only send if there is something to send
	if cl.MessageOut.length > 0 {
		(*cl.Connection).Write(cl.MessageOut.buffer)
		clearmsg(&cl.MessageOut)
	}
}

// Read all client names from disk, load their diskformats
// into memory. Add each to
//
// Called from initialize() at startup
func (q2a *RemoteAdminServer) LoadClients() {
	clientlist := q2a.ReadClientFile()
	cls := []Client{}
	for _, c := range clientlist {
		cl := Client{}
		err := cl.ReadFromDisk(c)
		if err != nil {
			continue
		}
		cls = append(cls, cl)
	}
	q2a.clients = cls
}

// Read a client "object" from disk and into memory.
//
// Called at startup for each client
func (cl *Client) ReadFromDisk(name string) error {
	sep := os.PathSeparator
	filename := fmt.Sprintf("%s%c%s.json", q2a.config.ClientDirectory, sep, name)
	filedata, err := os.ReadFile(filename)
	if err != nil {
		//log.Println("Problems with", name, "skipping")
		return errors.New("unable to read file")
	}
	sf := ClientDiskFormat{}
	err = json.Unmarshal([]byte(filedata), &sf)
	if err != nil {
		log.Println(err)
		return errors.New("unable to parse data")
	}

	addr := strings.Split(sf.Address, ":")
	if len(addr) == 2 {
		cl.Port, _ = strconv.Atoi(addr[1])
	} else {
		cl.Port = 27910
	}
	cl.IPAddress = addr[0]
	cl.Enabled = sf.Enabled
	cl.Owner = sf.Owner
	cl.Description = sf.Description
	cl.UUID = sf.UUID
	cl.Name = sf.Name
	cl.Verified = sf.Verified

	acls := []ClientRule{}
	for _, c := range sf.Rules {
		acl := ClientRule{}
		acl.Address = c.Address
		for _, ip := range c.Address {
			if !strings.Contains(ip, "/") { // no cidr notation, assuming /32
				ip += "/32"
			}
			_, netbinary, err := net.ParseCIDR(ip)
			if err != nil {
				log.Println("invalid cidr network in rule", c.ID, ip)
				continue
			}
			acl.Network = append(acl.Network, netbinary)
		}
		acl.Hostname = c.Hostname
		acl.Client = c.Client
		acl.Created = c.Created
		acl.Description = c.Description
		acl.Length = c.Length
		acl.Message = c.Message
		acl.Name = c.Name
		acl.Password = c.Password
		acl.StifleLength = c.StifleLength
		acl.Type = c.Type
		acl.UserInfoKey = c.UserInfoKey
		acl.UserinfoVal = c.UserinfoVal
		acls = append(acls, acl)
	}
	cl.Rules = SortRules(acls)
	return nil
}

// Write key portions of the Client struct
// to disk as JSON.
func (cl *Client) WriteToDisk(filename string) bool {
	rules := []ClientRuleFormat{}
	for _, r := range cl.Rules {
		rules = append(rules, r.ToDiskFormat())
	}

	df := ClientDiskFormat{
		UUID:        cl.UUID,
		Enabled:     cl.Enabled,
		Verified:    cl.Verified,
		Address:     fmt.Sprintf("%s:%d", cl.IPAddress, cl.Port),
		Name:        cl.Name,
		Owner:       cl.Owner,
		Description: cl.Description,
		Rules:       rules,
	}

	// name property is required, if not found, set random one
	if df.Name == "" {
		df.Name = hex.EncodeToString(RandomBytes(20))
	}

	filecontents, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		log.Println(err)
		return false
	}

	err = os.WriteFile(filename, filecontents, 0644)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

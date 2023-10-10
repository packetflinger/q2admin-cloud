// In this system, a "client" is a Quake 2 game server.
// They are servers to their connected players, but
// clients to us.
package client

import (
	"crypto/rsa"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
	"github.com/packetflinger/q2admind/util"
)

// This struct is partially populated by parsing disk a file
// on disk on init and the rest is filled in when the game
// server actually connects
type Client struct {
	ID          int                // this is the database index, remove later
	UUID        string             // random identifier
	Owner       string             // email addr
	Version     int                // what version are we running
	Name        string             // the teleport name
	Description string             // used in teleporting
	IPAddress   string             // used for teleporting
	Port        int                // used for teleporting
	Connected   bool               // is it currently connected to us?
	Verified    bool               // client owner proved they're the owner
	CurrentMap  string             // what map is currently running
	Enabled     bool               // actually use it
	Connection  *net.Conn          // the tcp connection
	Players     []Player           // all the connected players
	PlayerCount int                // len(Players)
	MaxPlayers  int                // total number
	Message     util.MessageBuffer // incoming byte stream
	MessageOut  util.MessageBuffer // outgoing byte stream
	Encrypted   bool               // are the messages AES encrypted?
	Trusted     bool               // signature challenge verified
	PublicKey   *rsa.PublicKey     // supplied by owner via website
	AESKey      []byte             // 16 (128bit)
	AESIV       []byte             // 16 bytes (CBC)
	Rules       []*pb.Rule         // bans, mutes, etc
	PingCount   int                // how many pings client has seen
	WebSockets  []*websocket.Conn  // slice of web clients
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
	//Rules []ClientRuleFormat `json:"Controls"`
}

// Reads the client textproto file. This file contains
// every client we expect to interacat with.
//
// Called from initialize() at startup
/*
func (s *RemoteAdminServer) ReadClientFile() ([]string, error) {
	clients := []string{}
	clientspb := pb.ClientList{}
	contents, err := os.ReadFile(s.config.GetClientFile())
	if err != nil {
		return clients, err
	}
	err = prototext.Unmarshal(contents, &clientspb)
	if err != nil {
		return clients, err
	}

	clients = clientspb.GetClient()
	return clients, nil
}
*/

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

// Read all client names from disk, load their data
// into memory. Add each to the client list.
//
// Called from initialize() at startup
func LoadClients(filename string) ([]Client, error) {
	clients := []Client{}
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
		client, err := (&Client{}).LoadSettings(c)
		if err != nil {
			continue
		}
		client.Rules, err = client.FetchRules()
		if err != nil {
			log.Println(err)
		}
		clients = append(clients, client)
	}
	return clients, nil
}

// Read settings file for client from disk and make a *Client struct
// from them.
func (cl *Client) LoadSettings(name string) (Client, error) {
	var client Client
	filename := path.Join("clients", name, "settings")
	contents, err := os.ReadFile(filename)
	if err != nil {
		return client, err
	}
	cls := pb.Clients{}
	err = prototext.Unmarshal(contents, &cls)
	if err != nil {
		return client, err
	}

	for _, c := range cls.GetClient() {
		if c.GetName() != name {
			continue
		}
		client.Name = c.GetName()
		client.Owner = c.GetOwner()
		client.Description = c.GetDescription()
		client.UUID = c.GetUuid()

		tokens := strings.Split(c.GetAddress(), ":")
		if len(tokens) == 2 {
			client.Port, err = strconv.Atoi(tokens[1])
			if err != nil {
				client.Port = 27910
			}
		} else {
			client.Port = 27910
		}
		client.IPAddress = tokens[0]
	}
	return client, nil
}

// Read rules from disk and return a slice of them
func (cl *Client) FetchRules() ([]*pb.Rule, error) {
	var rules []*pb.Rule
	filename := path.Join("clients", cl.Name, "rules")
	contents, err := os.ReadFile(filename)
	if err != nil {
		return rules, err
	}
	rl := pb.Rules{}
	err = prototext.Unmarshal(contents, &rl)
	if err != nil {
		return rules, err
	}
	rules = rl.GetRule()
	return rules, nil
}

/*
// Read a client "object" from disk and into memory.
//
// Called at startup for each client
func (cl *Client) LoadFromDisk(name string) error {
	filename := path.Join(q2a.config.ClientDirectory, name)
	filedata, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	sf := pb.Clients{}
	err = prototext.Unmarshal(filedata, &sf)
	if err != nil {
		return err
	}

	// in case more than 1 client is specified in this file
	for _, c := range sf.GetClient() {
		tokens := strings.Split(c.GetAddress(), ":")
		if len(tokens) == 2 {
			cl.Port, err = strconv.Atoi(tokens[1])
			if err != nil {
				cl.Port = 27910
			}
		} else {
			cl.Port = 27910
		}
		cl.IPAddress = tokens[0]
		//cl.Enabled = sf.
		cl.Owner = c.GetOwner()
		cl.Description = c.GetDescription()
		cl.UUID = c.GetUuid()
		cl.Name = c.GetName()
		cl.Verified = c.GetVerified()

		fmt.Println("reading", cl.Name)
	}

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
*/

/*
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
*/

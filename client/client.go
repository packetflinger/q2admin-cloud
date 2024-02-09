// In this system, a "client" is a Quake 2 game server.
// They are servers to their connected players, but
// clients to us.
package client

import (
	"crypto/rsa"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/util"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// This struct is partially populated by parsing disk a file
// on disk on init and the rest is filled in when the game
// server actually connects
type Client struct {
	ID          int                   // this is the database index, remove later
	UUID        string                // random identifier
	Owner       string                // email addr
	Version     int                   // what version are we running
	Name        string                // the teleport name
	Description string                // used in teleporting
	IPAddress   string                // used for teleporting
	Port        int                   // used for teleporting
	Connected   bool                  // is it currently connected to us?
	Verified    bool                  // client owner proved they're the owner
	CurrentMap  string                // what map is currently running
	Enabled     bool                  // actually use it
	Connection  *net.Conn             // the tcp connection
	Players     []Player              // all the connected players
	PlayerCount int                   // len(Players)
	MaxPlayers  int                   // total number
	Message     message.MessageBuffer // incoming byte stream
	MessageOut  message.MessageBuffer // outgoing byte stream
	Encrypted   bool                  // are the messages AES encrypted?
	Trusted     bool                  // signature challenge verified
	PublicKey   *rsa.PublicKey        // supplied by owner via website
	AESKey      []byte                // 16 (128bit)
	AESIV       []byte                // 16 bytes (CBC)
	Rules       []*pb.Rule            // bans, mutes, etc
	PingCount   int                   // how many pings client has seen
	WebSockets  []*websocket.Conn     // slice of web clients
	Log         *log.Logger           // log stuff here
	LogFile     *os.File              // pointer to file so we can close when client disconnects
	APIKeys     *pb.ApiKeys           // keys generated for accessing this client
}

// Each client keeps track of the websocket for people "looking at it".
// When they close the browser or logout, remove the pointer
// to that socket
func (cl *Client) DeleteWebSocket(sock *websocket.Conn) {
	location := -1
	// find it's index first
	for i := range cl.WebSockets {
		if cl.WebSockets[i] == sock {
			location = i
			break
		}
	}

	// wasn't found, forget it
	if location == -1 {
		return
	}

	tempws := cl.WebSockets[0:location]
	tempws = append(tempws, cl.WebSockets[location+1:]...)
	cl.WebSockets = tempws
}

// Read rules from disk and return a slice of them
func (cl *Client) FetchRules() ([]*pb.Rule, error) {
	var rules []*pb.Rule
	filename := path.Join("clients", cl.Name, "rules.pb")
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
	filename := path.Join("clients", name, "settings.pb")
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

// Send all messages in the outgoing queue to the client (gameserver)
func (cl *Client) SendMessages() {
	if !cl.Connected {
		return
	}

	// keys have been exchanged, encrypt the message
	if cl.Trusted && cl.Encrypted {
		cipher := crypto.SymmetricEncrypt(
			cl.AESKey,
			cl.AESIV,
			cl.MessageOut.Buffer[:cl.MessageOut.Index])
		cl.MessageOut = message.NewMessageBuffer(cipher)
	}

	// only send if there is something to send
	if len(cl.MessageOut.Buffer) > 0 {
		(*cl.Connection).Write(cl.MessageOut.Buffer)
		(&cl.MessageOut).Reset()
	}
}

// Send the txt string to all the websockets listening
func (cl *Client) SendToWebsiteFeed(txt string, decoration int) {
	now := util.GetTimeNow()

	colored := ""
	switch decoration {
	/*
		case api.FeedChat:
			colored = now + " \\\\e[32m" + txt + "\\\\e[0m"
		case api.FeedJoinPart:
			colored = now + " \\\\e[33m\\\\e[42m" + txt + "\\\\e[0m"
	*/
	default:
		colored = now + " " + txt
	}

	sockets := cl.WebSockets
	for i := range sockets {
		err := sockets[i].WriteMessage(1, []byte(colored))
		if err != nil {
			log.Println(err)
			cl.DeleteWebSocket(cl.WebSockets[i])
		}
	}
}

// convert to
func (cl *Client) ToProto() *pb.Client {
	p := pb.Client{}
	p.Address = fmt.Sprintf("%s:%d", cl.IPAddress, cl.Port)
	p.Name = cl.Name
	p.Uuid = cl.UUID
	p.Description = cl.Description
	p.Owner = cl.Owner
	p.Verified = cl.Verified
	return &p
}

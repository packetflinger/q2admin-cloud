// In this system, a "client" is a Quake 2 game server.
// They are servers to their connected players, but
// clients to us.
package client

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

	"github.com/gorilla/websocket"
	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/util"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// This struct is partially populated by parsing a config file
// on disk during init and the rest is filled in when the game
// server actually connects
type Client struct {
	ID          int                   // this is the database index, remove later
	UUID        string                // random identifier
	Owner       string                // email addr
	Version     int                   // q2admin library version
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
	CryptoKey   crypto.EncryptionKey  // AES 128 CBC
	Rules       []*pb.Rule            // bans, mutes, etc
	PingCount   int                   // how many pings client has seen
	WebSockets  []*websocket.Conn     // slice of web clients
	Log         *log.Logger           // log stuff here
	LogFile     *os.File              // pointer to file so we can close when client disconnects
	APIKeys     *pb.ApiKeys           // keys generated for accessing this client
	Path        string                // the fs path for this client
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
	filename := path.Join(cl.Path, "rules.pb")
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

// Read settings file for client from disk and make a *Client struct
// from them.
func LoadSettings(name string, clientsDir string) (Client, error) {
	var client Client
	filename := path.Join(clientsDir, name, "settings.pb")
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
		client.Path = path.Join(clientsDir, client.Name)

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
func (cl *Client) GetPlayerFromPrint(txt string) ([]*Player, error) {
	var players []*Player
	var name string

	count := strings.Count(txt, ": ") // note the space
	if count == 0 {
		return nil, errors.New("no name in print")
	} else {
		tokens := strings.Split(txt, ": ")
		if len(tokens) > 1 {
			name = tokens[0]
		}

		fmt.Println(name)
		for i, p := range cl.Players {
			if p.Name == name {
				players = append(players, &cl.Players[i])
			}
		}
	}

	return players, nil
}

// Send all messages in the outgoing queue to the client (gameserver)
func (cl *Client) SendMessages() {
	if !cl.Connected {
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

// Find all players that match the name provided. Multiple players
// are allowed to have the same name at the same time, this will
// return all of them.
func (cl *Client) PlayersByName(name string) ([]*Player, error) {
	var players []*Player
	if name == "" {
		return players, errors.New("blank name argument")
	}
	for i, p := range cl.Players {
		if p.Name == name {
			players = append(players, &cl.Players[i])
		}
	}
	return players, nil
}

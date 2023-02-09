// In this system, a "client" is a Quake 2 game server.
// They are servers to their connected players, but
// clients to us.
package main

import (
	"crypto/rsa"
	"errors"
	"log"
	"net"
	"os"
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
func (c Config) ReadClientFile() []string {
	contents, err := os.ReadFile(c.ClientsFile)
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

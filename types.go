package main

import (
	"crypto/rsa"
	"net"

	"github.com/gorilla/websocket"
)

//
// Use a custom buffer struct to keep track of where
// we are in the stream of bytes internally
//
type MessageBuffer struct {
	buffer []byte
	index  int
	length int // maybe not needed
}

//
// This is a Quake 2 Gameserver, and also a client to us.
//
// This struct is partially populated by parsing disk a file
// on disk on init and the rest is filled in when the game
// server actually connects
//
type Client struct {
	ID          int // this is the database index
	UUID        string
	Owner       string // email addr
	Version     int    // what version are we running
	Name        string
	Description string // used in teleporting
	IPAddress   string // used for teleporting
	Port        int    // used for teleporting
	Connected   bool   // is it currently connected to us?
	Verified    bool
	CurrentMap  string
	Enabled     bool
	Connection  *net.Conn
	Players     []Player
	PlayerCount int
	MaxPlayers  int
	Message     MessageBuffer  // incoming byte stream
	MessageOut  MessageBuffer  // outgoing byte stream
	Encrypted   bool           // are the messages AES encrypted?
	Trusted     bool           // signature challenge verified
	PublicKey   *rsa.PublicKey // supplied by owner via website
	AESKey      []byte         // 16 (128bit)
	AESIV       []byte         // 16 bytes (CBC)
	Rules       []ClientRule   // bans, mutes, etc
	PingCount   int
	WebSockets  []*websocket.Conn
}

//
// "This" admin server
//
type RemoteAdminServer struct {
	Users      []User          // website users
	config     Config          // global config
	clients    []Client        // managed quake 2 servers
	access     []UserAccess    // permissions
	rules      []ClientRule    // bans/mutes/etc
	privatekey *rsa.PrivateKey // private to us
	publickey  *rsa.PublicKey  // known to clients
}

//
// The config file once parsed
//
type Config struct {
	Address         string `json:"address"`
	Port            int    `json:"port"`
	Database        string `json:"database"`
	DBString        string `json:"dbstring"`
	PrivateKey      string `json:"privatekey"`
	APIPort         int    `json:"apiport"`
	Debug           int    `json:"debug"`
	APIEnabled      int    `json:"enableapi"`
	ClientsFile     string `json:"clientsfile"`
	ClientDirectory string `json:"clientdir"` // folder for json files
}

//
// Websocket
//
type WebSocketConnection struct {
	Connected bool
	Socket    *websocket.Conn
}

type ClientRule struct {
	ID           string
	Type         string // ["ban","mute","stifle","msg"]
	Address      string
	Network      *net.IPNet
	Name         []string // optional
	Client       []string // optional
	UserInfoKey  []string // optional
	UserinfoVal  []string // optional
	Description  string
	Message      string
	Password     string
	Exact        bool
	StifleLength int   // secs
	Created      int64 // unix timestamp
	Length       int64 // secs after Created before expiring
}

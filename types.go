package main

import (
	"crypto/rsa"
	"net"
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
// This struct is partially populated by the database on
// init and the rest is filled in when the game server
// actually connects
//
type Server struct {
	ID         int // this is the database index
	UUID       string
	Owner      int // user id from database
	Version    int // what version are we running
	Name       string
	IPAddress  string // used for teleporting
	Port       int    // used for teleporting
	Connected  bool   // is it currently connected to us?
	CurrentMap string
	Enabled    bool
	Connection *net.Conn
	Players    []Player
	MaxPlayers int
	Message    MessageBuffer  // incoming byte stream
	MessageOut MessageBuffer  // outgoing byte stream
	Encrypted  bool           // are the messages AES encrypted?
	Trusted    bool           // signature challenge verified
	PublicKey  *rsa.PublicKey // supplied by owner via website
	AESKey     []byte         // 16 (128bit)
	AESIV      []byte         // 16 bytes (CBC)
	Bans       []Ban
	PingCount  int
}

//
// "This" admin server
//
type AdminServer struct {
	privatekey *rsa.PrivateKey
	publickey  *rsa.PublicKey
}

//
// The config file once parsed
//
type Config struct {
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Database   string `json:"database"`
	DBString   string `json:"dbstring"`
	PrivateKey string `json:"privatekey"`
	APIPort    int    `json:"apiport"`
	Debug      int    `json:"debug"`
	APIEnabled int    `json:"enableapi"`
}
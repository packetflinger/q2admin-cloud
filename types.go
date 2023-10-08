package main

import (
	"crypto/rsa"

	"github.com/gorilla/websocket"
	pb "github.com/packetflinger/q2admind/proto"
)

// Use a custom buffer struct to keep track of where
// we are in the stream of bytes internally
type MessageBuffer struct {
	buffer []byte
	index  int
	length int // maybe not needed
}

// "This" admin server
type RemoteAdminServer struct {
	Users []User // website users
	//config     Config          // global config
	config     pb.Config
	clients    []Client        // managed quake 2 servers
	access     []UserAccess    // permissions
	rules      []*pb.Rule      // bans/mutes/etc
	privatekey *rsa.PrivateKey // private to us
	publickey  *rsa.PublicKey  // known to clients
	maintcount int             // total maintenance runs
}

/*
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
	ClientDirectory string `json:"clientdir"`  // folder for json files
	UsersFile       string `json:"usersfile"`  // web users
	AccessFile      string `json:"accessfile"` // their permissions
	OAuthFile       string `json:"oauthfile"`  // api credentials
	MaintenanceTime int    `json:"mainttime"`  // seconds to sleep
}
*/

// Websocket
type WebSocketConnection struct {
	Connected bool
	Socket    *websocket.Conn
}

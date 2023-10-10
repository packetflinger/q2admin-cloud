package main

import (
	"github.com/gorilla/websocket"
)

// Use a custom buffer struct to keep track of where
// we are in the stream of bytes internally
type MessageBuffer struct {
	buffer []byte
	index  int
	length int // maybe not needed
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

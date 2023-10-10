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

// Websocket
type WebSocketConnection struct {
	Connected bool
	Socket    *websocket.Conn
}

package api

import (
	"github.com/gorilla/websocket"
)

// Websocket
type WebSocketConnection struct {
	Connected bool
	Socket    *websocket.Conn
}


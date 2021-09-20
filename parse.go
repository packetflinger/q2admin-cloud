package main

import (
    "log"
    "strconv"
)

/**
 * Loop through all the data from the client
 * and act accordingly
 */
func ParseMessage(srv *Server) {
    msg := &srv.message
    for {
        if msg.index >= len(msg.buffer) {
            break
        }

        switch b := ReadByte(msg); b {
        case CMDPing:
            Pong(srv)

        case CMDPrint:
            ParsePrint(srv)

        case CMDMap:
            ParseMap(srv)

        case CMDPlayerList:
            ParsePlayerlist(srv)

        case CMDConnect:
            ParseConnect(srv)

        case CMDDisconnect:
            ParseDisconnect(srv)

        case CMDCommand:
            ParseCommand(srv)

        case CMDFrag:
            ParseFrag(srv)
        }
    }
}

/**
 * A player was fragged.
 * Only two bytes are sent: the clientID of the victim,
 * and of the attacker
 */
func ParseFrag(srv *Server) {
    v := ReadByte(&srv.message)
    a := ReadByte(&srv.message)

    //victim := findplayer(srv.players, int(v))

    log.Printf("[%s/FRAG] %d > %d\n", srv.name, a, v)
}

/**
 * Received a ping from a client, send a pong to show we're alive
 */
func Pong(srv *Server) {
    //log.Printf("[%s/PING]\n", srv.name)
    WriteByte(SCMDPong, &srv.messageout)
}

/**
 * A print was sent by the server.
 * 1 byte: print level
 * string: the actual message
 */
func ParsePrint(srv *Server) {
    level := ReadByte(&srv.message)
    text := ReadString(&srv.message)
    log.Printf("[%s/PRINT] (%d) %s\n", srv.name, level, text)
}

/**
 * A player connected to the a q2 server
 */
func ParseConnect(srv *Server) {
    clientnum := ReadByte(&srv.message)
    userinfo := ReadString(&srv.message)
    info := UserinfoMap(userinfo)
    port, _ := strconv.Atoi(info["port"])
    fov, _ := strconv.Atoi(info["fov"])
    newplayer := Player{
        clientid: int(clientnum),
        userinfo: userinfo,
        name: info["name"],
        ip: info["ip"],
        port: port,
        fov: fov,
    }

    srv.players = append(srv.players, newplayer)
    log.Printf("[%s/CONNECT] (%d) %s - %s\n", srv.name, clientnum, info["name"], info["ip"])
}

/**
 * A player disconnected from a q2 server
 */
func ParseDisconnect(srv *Server) {
    clientnum := ReadByte(&srv.message)
    srv.players = removeplayer(srv.players, int(clientnum))
    log.Printf("[%s/DISCONNECT] (%d)\n", srv.name, clientnum)
}

/**
 * Server told us what map is currently running. Typically happens
 * when the map changes
 */
func ParseMap(srv *Server) {
    mapname := ReadString(&srv.message)
    srv.currentmap = mapname
    log.Printf("[%s/MAP] %s\n", srv.name, srv.currentmap)
}

func ParsePlayerlist(srv *Server) {
    count := ReadByte(&srv.message)
    log.Printf("[%s/PLAYERLIST] %d\n", srv.name, count)
    for i:=0; i<int(count); i++ {
        ParsePlayer(srv)
    }
}

func ParsePlayer(srv *Server) {
    clientnum := ReadByte(&srv.message)
    userinfo := ReadString(&srv.message)
    log.Printf("[%s/PLAYER] (%d) %s\n", srv.name, clientnum, userinfo)
}

func ParseCommand(srv *Server) {
    cmd := ReadByte(&srv.message)
    switch cmd {
    case PCMDTeleport:
        Teleport(srv)

    case PCMDInvite:
        Invite(srv)
    }
}

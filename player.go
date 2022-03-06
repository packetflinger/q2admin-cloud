package main

import "strings"

/**
 * Each player on a game server has one of these.
 * Each game server has a slice of all current players
 */
type Player struct {
	clientid         int
	name             string
	userinfo         string
	hash             string
	frags            int
	deaths           int
	suicides         int
	teleports        int
	lastteleport     int64 // actually going
	lastteleportlist int64 // viewing the big list of destinations
	invites          int
	lastinvite       int64
	invitesavailable int
	ip               string
	port             int
	fov              int
}

/**
 * Get a pointer to a player based on a client number
 */
func findplayer(players []Player, cl int) *Player {
	for i, p := range players {
		if p.clientid == cl {
			return &players[i]
		}
	}

	return nil
}

/**
 * Remove a player from the players slice (used when player quits)
 */
func removeplayer(players []Player, cl int) []Player {
	var index int
	for i, pl := range players {
		if pl.clientid == cl {
			index = i
			break
		}
	}

	return append(players[:index], players[index+1:]...)
}

/**
 * Send a message to every player on the server
 */
func SayEveryone(srv *Server, level int, text string) {
	WriteByte(SCMDSayAll, &srv.messageout)
	WriteByte(byte(level), &srv.messageout)
	WriteString(text, &srv.messageout)
}

/**
 * Send a message to a particular player
 */
func SayPlayer(srv *Server, client int, level int, text string) {
	WriteByte(SCMDSayClient, &srv.messageout)
	WriteByte(byte(client), &srv.messageout)
	WriteByte(byte(level), &srv.messageout)
	WriteString(text, &srv.messageout)
}

/**
 * Take a back-slash delimited string of userinfo and return
 * a key/value map
 */
func UserinfoMap(ui string) map[string]string {
	info := make(map[string]string)
	if ui == "" {
		return info
	}

	data := strings.Split(ui[1:], "\\")

	for i := 0; i < len(data); i += 2 {
		info[data[i]] = data[i+1]
	}

	// special case: split the IP value into IP and Port
	ip := info["ip"]
	ipport := strings.Split(ip, ":")
	if len(ipport) >= 2 {
		info["port"] = ipport[1]
		info["ip"] = ipport[0]
	}

	return info
}

package main

import (
	"crypto/md5"
	"fmt"
	"strings"
)

/**
 * Each player on a game server has one of these.
 * Each game server has a slice of all current players
 */
type Player struct {
	ClientID         int // ID on the gameserver (0-maxplayers)
	Database_ID      int64
	Name             string
	Version          string // q2 client flavor + version
	Userinfo         string
	UserinfoMap      map[string]string
	Hash             string
	Frags            int
	Deaths           int
	Suicides         int
	Teleports        int
	LastTeleport     int64 // actually going
	LastTeleportList int64 // viewing the big list of destinations
	Invites          int
	LastInvite       int64
	InvitesAvailable int
	IP               string
	Port             int
	FOV              int
	ConnectTime      int64
	Rules            []ClientRule // rules that match this player
}

/**
 * Get a pointer to a player based on a client number
 */
func (cl *Client) FindPlayer(client int) *Player {
	if !cl.ValidPlayerID(client) {
		return nil
	}

	p := &cl.Players[client]

	if p.ConnectTime > 0 {
		return p
	}

	return nil
}

/**
 * A player hash is a way of uniquely identifiying a player.
 *
 * It's the first 16 characters of an MD5 hash of their
 * name + skin + fov + partial IP. The idea is to identify
 * players with the same name as different people, so someone can't
 * impersonate someone else and tank their stats.
 *
 * Players can specify a player hash in their Userinfo rather than
 * having one generated. This way they can use different names and
 * still have their stats follow them.
 *
 * To specify a player hash from your q2 config:
 * set phash "<hash here>" u
 */
func LoadPlayerHash(player *Player) {
	var database_id int64

	phash := player.UserinfoMap["phash"]
	if phash != "" {
		player.Hash = phash
	} else {
		ipslice := strings.Split(player.IP, ".")
		ip := fmt.Sprintf("%s.%s.%s", ipslice[0], ipslice[1], ipslice[2])

		pt := []byte(fmt.Sprintf(
			"%s-%s-%s-%s",
			player.Name,
			player.UserinfoMap["skin"],
			player.UserinfoMap["fov"],
			ip,
		))

		hash := md5.Sum(pt)
		player.Hash = fmt.Sprintf("%x", hash[:8])
	}

	database_id = int64(GetPlayerIdFromHash(player.Hash))
	if database_id > 0 {
		player.Database_ID = database_id
		return
	}

	database_id = InsertPlayer(player)
	player.Database_ID = database_id
}

/**
 * Check if a client ID is valid for a particular server context,
 * does not care if a valid player structure is located there or not
 */
func (cl *Client) ValidPlayerID(client int) bool {
	return client >= 0 && client < len(cl.Players)
}

/**
 * Remove a player from the players slice (used when player quits)
 */
func (cl *Client) RemovePlayer(client int) {
	if cl.ValidPlayerID(client) {
		cl.Players[client] = Player{}
		cl.PlayerCount--
	}
}

/**
 * Send a message to every player on the server
 */
func (cl *Client) SayEveryone(level int, text string) {
	WriteByte(SCMDSayAll, &cl.MessageOut)
	WriteByte(byte(level), &cl.MessageOut)
	WriteString(text, &cl.MessageOut)
}

/**
 * Send a message to a particular player
 */
func (cl *Client) SayPlayer(p *Player, level int, text string) {
	WriteByte(SCMDSayClient, &cl.MessageOut)
	WriteByte(byte(p.ClientID), &cl.MessageOut)
	WriteByte(byte(level), &cl.MessageOut)
	WriteString(text, &cl.MessageOut)
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

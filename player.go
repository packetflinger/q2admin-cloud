package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
)

// Each player on a game server has one of these.
// Each game server has a slice of all current players
type Player struct {
	ClientID         int // ID on the gameserver (0-maxplayers)
	Database_ID      int64
	Name             string
	Version          string // q2 client flavor + version
	Userinfo         string
	UserinfoMap      map[string]string
	UserInfoHash     string // md5 hash for checking if UI changed
	Cookie           string // a unique value to identify players
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
	Hostname         string
	FOV              int
	ConnectTime      int64
	Rules            []ClientRule // rules that match this player
	Stifled          bool
	StifleLength     int     // seconds
	Client           *Client // circular ref
}

// Get a pointer to a player based on a client number
func (cl *Client) FindPlayer(client int) *Player {
	if !cl.ValidPlayerID(client) {
		return nil
	}

	for i, p := range cl.Players {
		if p.ClientID == client && p.ConnectTime > 0 {
			return &cl.Players[i]
		}
	}

	return nil
}

// A player hash is a way of uniquely identifiying a player.
//
// It's the first 16 characters of an MD5 hash of their
// name + skin + fov + partial IP. The idea is to identify
// players with the same name as different people, so someone can't
// impersonate someone else and tank their stats.
//
// Players can specify a player hash in their Userinfo rather than
// having one generated. This way they can use different names and
// still have their stats follow them.
//
// To specify a player hash from your q2 config:
// set phash "<hash here>" u
//
// Called from ParsePlayer()
func (player *Player) LoadPlayerHash() {
	var database_id int64

	phash := player.UserinfoMap["phash"]
	if phash != "" {
		player.UserInfoHash = phash
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
		player.UserInfoHash = fmt.Sprintf("%x", hash[:8])
	}

	database_id = int64(GetPlayerIdFromHash(player.UserInfoHash))
	if database_id > 0 {
		player.Database_ID = database_id
		return
	}

	database_id = InsertPlayer(player)
	player.Database_ID = database_id
}

// Check if a client ID is valid for a particular server context,
// does not care if a valid player structure is located there or not
func (cl *Client) ValidPlayerID(client int) bool {
	return client >= 0 && client < len(cl.Players)
}

// Remove a player from the players slice (used when player quits)
func (cl *Client) RemovePlayer(client int) {
	if !cl.ValidPlayerID(client) {
		log.Printf("invalid client number (%d) when removing player\n", client)
		return
	}

	for i := range cl.Players {
		if cl.Players[i].ClientID == client {
			cl.Players[i] = Player{}
			cl.PlayerCount--
			return
		}
	}
}

// Send a message to every player on the server
func (cl *Client) SayEveryone(level int, text string) {
	if text == "" {
		return
	}
	WriteByte(SCMDSayAll, &cl.MessageOut)
	WriteByte(byte(level), &cl.MessageOut)
	WriteString(text, &cl.MessageOut)
}

// Send a message to a particular player
func (cl *Client) SayPlayer(p *Player, level int, text string) {
	if text == "" {
		return
	}

	text += "\n"
	WriteByte(SCMDSayClient, &cl.MessageOut)
	WriteByte(byte(p.ClientID), &cl.MessageOut)
	WriteByte(byte(level), &cl.MessageOut)
	WriteString(text, &cl.MessageOut)
}

// Take a back-slash delimited string of userinfo and return
// a key/value map
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

// A player connected, save them in the database
//
// Called from ParseConnect()
func (cl *Client) LogPlayer(pl *Player) {
	s := "INSERT INTO player (server, name, ip, hash, userinfo, connect_time) VALUES (?,?,?,?,?,?)"
	_, err := DB.Exec(
		s,
		cl.UUID,
		pl.Name,
		pl.IP,
		pl.UserInfoHash,
		pl.Userinfo,
		GetUnixTimestamp(),
	)
	if err != nil {
		log.Println(err)
	}
}

// Find the first player using the provided name
// on this particular client.
//
// Called from CalculateDeath()
func (cl *Client) FindPlayerByName(name string) *Player {
	for i, p := range cl.Players {
		if p.Name == name {
			return &cl.Players[i]
		}
	}

	return nil
}

// Setup a new cookie on a player
//
// Player cookies are a dirty and not terribly effective way of
// uniquely identifying players. Original Q2 made no effort to
// ID players other than their client number. Every player in the
// game can have the same name, skin, etc. The player's IP address
// was the only way to really differentiate them from other players.
// Now in the age of VPNs, an malicious player can get banned, and
// reconnect on a VPN with different IP, different name, etc, and
// continue being abusive.
//
// The idea of a player cookie is a persistent unique identifier.
// PlayerX can reconnect with a different name on a different IP
// with a different client and still be identified. This is great
// for tracking statistics and disciplinary actions (muting/banning
// shitheads).
func (p *Player) SetupCookie() {
	value := hex.EncodeToString(RandomBytes(12)) // random ID

	// "modern" clients (q2pro, r1q2) support seta for archive vars
	a := fmt.Sprintf("seta cl_cookie %s", value)

	// ancient clients (3.2[01]) require old format "set name value a"
	//a_old := fmt.Sprintf("set cl_cookie %s a", value)

	u := "setu cl_cookie $cl_cookie"

	// tell player to write the var to local .cfg file for persistence
	(p.Client).StuffPlayer(*p, a)

	// tell player to add var to their userinfo string. This will
	// trigger a ClientUserinfoChanged() call on the game server
	(p.Client).StuffPlayer(*p, u)
}

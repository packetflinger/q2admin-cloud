package client

import (
	"crypto/md5"
	"fmt"
	"log"
	"strings"

	pb "github.com/packetflinger/q2admind/proto"
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
	VPN              bool
	FOV              int
	ConnectTime      int64
	Rules            []*pb.Rule // rules that match this player
	Stifled          bool
	StifleLength     int     // seconds
	Client           *Client // circular ref
	Muted            bool    // is this player muted?
	FloodInfo        *pb.FloodInfo
}

// Get a pointer to a player based on a client number
func (cl *Client) FindPlayer(client int) (*Player, error) {
	if !cl.ValidPlayerID(client) {
		return nil, fmt.Errorf("invalid player id %q", client)
	}
	for i, p := range cl.Players {
		if p.ClientID == client && p.ConnectTime > 0 {
			return &cl.Players[i], nil
		}
	}
	return nil, fmt.Errorf("player %q not found", client)
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
//
// TODO: figure out the database-ness
func (player *Player) LoadPlayerHash() {
	//var database_id int64

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

	/*
		database_id = int64(GetPlayerIdFromHash(player.UserInfoHash))
		if database_id > 0 {
			player.Database_ID = database_id
			return
		}

		database_id = InsertPlayer(player)
		player.Database_ID = database_id
	*/
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

// Dump will return a string containing information about the player
func (p *Player) Dump() string {
	var out string
	out += fmt.Sprintf("%s's information\n", p.Name)
	out += fmt.Sprintf("  %-20s%s\n", "name:", p.Name)
	out += fmt.Sprintf("  %-20s%s\n", "ip:", p.IP)
	out += fmt.Sprintf("  %-20s%s\n", "client:", p.Version)
	out += fmt.Sprintf("  %-20s%v\n", "vpn:", p.VPN)
	out += fmt.Sprintf("  %-20s\n", "userinfo:")
	for k, v := range p.UserinfoMap {
		out += fmt.Sprintf("%15s = %s\n", k, v)
	}
	out += fmt.Sprintf("  %-20s\n", "rules matched:")
	for _, r := range p.Rules {
		out += fmt.Sprintf("%25s\n", strings.Join(r.GetDescription(), " "))
	}
	return out
}

package main

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

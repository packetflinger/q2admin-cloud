package server

import (
	"fmt"
	"log"
	"time"

	"github.com/packetflinger/q2admind/client"
)

// Called when a player issues the invite command in-game. This will print a
// message on all connected gameservers (configured to accept invites).
func Invite(cl *client.Client) {
	if cl == nil {
		return
	}
	client := (&cl.Message).ReadByte()
	text := (&cl.Message).ReadString()

	p, err := cl.FindPlayer(int(client))
	if err != nil {
		cl.Log.Println("invite problem:", err)
		cl.SSHPrintln("invite problem: " + err.Error())
	}
	if !cl.AllowInvite {
		SayPlayer(cl, p, PRINT_CHAT, "This server doesn't allow invites.")
		return
	}
	if cl.Invites.Tokens == 0 {
		SayPlayer(cl, p, PRINT_CHAT, "No invite tokens remaining, you'll have to wait a few minutes.")
		return
	}
	log.Printf("[%s/INVITE/%s] %s\n", cl.Name, p.Name, text)

	now := time.Now().Unix()
	invtime := now - p.LastInvite

	if p.InvitesAvailable == 0 {
		if invtime > 600 {
			p.InvitesAvailable = 3
		} else {
			txt := fmt.Sprintf("You have no more invite tokens available, wait %d seconds\n", 600-invtime)
			SayPlayer(cl, p, PRINT_HIGH, txt)
			return
		}
	} else {
		if invtime < 30 {
			txt := fmt.Sprintf("Invite used too recently, wait %d seconds\n", 30-invtime)
			SayPlayer(cl, p, PRINT_HIGH, txt)
			return
		}
	}

	inv := fmt.Sprintf("%s invites you to play at %s (%s:%d)", p.Name, cl.Name, cl.IPAddress, cl.Port)
	for _, s := range srv.clients {
		if s.Enabled && s.Connected && s.AllowInvite {
			SayEveryone(&s, PRINT_CHAT, inv)
		}
	}

	p.Invites++
	p.LastInvite = now
	p.InvitesAvailable--
	cl.Invites.Tokens--
	cl.Invites.UseCount++
}

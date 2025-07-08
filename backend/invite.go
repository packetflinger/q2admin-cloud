package backend

import (
	"fmt"
	"log"
	"time"

	"github.com/packetflinger/q2admind/frontend"
)

// Called when a player issues the invite command in-game. This will print a
// message on all connected frontends (configured to accept invites).
func Invite(fe *frontend.Frontend) {
	if fe == nil {
		return
	}
	client := (&fe.Message).ReadByte()
	text := (&fe.Message).ReadString()

	p, err := fe.FindPlayer(int(client))
	if err != nil {
		fe.Log.Println("invite problem:", err)
		fe.SSHPrintln("invite problem: " + err.Error())
	}
	if !fe.AllowInvite {
		SayPlayer(fe, p, PRINT_CHAT, "This server doesn't allow invites.")
		return
	}
	if fe.Invites.Tokens == 0 {
		SayPlayer(fe, p, PRINT_CHAT, "No invite tokens remaining, you'll have to wait a few minutes.")
		return
	}
	log.Printf("[%s/INVITE/%s] %s\n", fe.Name, p.Name, text)

	now := time.Now().Unix()
	invtime := now - p.LastInvite

	if p.InvitesAvailable == 0 {
		if invtime > 600 {
			p.InvitesAvailable = 3
		} else {
			txt := fmt.Sprintf("You have no more invite tokens available, wait %d seconds\n", 600-invtime)
			SayPlayer(fe, p, PRINT_HIGH, txt)
			return
		}
	} else {
		if invtime < 30 {
			txt := fmt.Sprintf("Invite used too recently, wait %d seconds\n", 30-invtime)
			SayPlayer(fe, p, PRINT_HIGH, txt)
			return
		}
	}

	inv := fmt.Sprintf("%s invites you to play at %s (%s:%d)", p.Name, fe.Name, fe.IPAddress, fe.Port)
	for _, s := range be.frontends {
		if s.Enabled && s.Connected && s.AllowInvite {
			SayEveryone(&s, PRINT_CHAT, inv)
		}
	}

	p.Invites++
	p.LastInvite = now
	p.InvitesAvailable--
	fe.Invites.Tokens--
	fe.Invites.UseCount++
}

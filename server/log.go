package server

import (
	"log"

	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/util"
)

// A player said something, record to use against them later
func LogChat(cl *client.Client, chat string) {
	s := "INSERT INTO chat (uuid, chat_time, chat) VALUES (?,?,?)"
	_, err := DB.Exec(s, cl.UUID, util.GetUnixTimestamp(), chat)
	if err != nil {
		log.Println(err)
	}
}

// Save frags for stats
func LogFrag(cl *client.Client, victim int, attacker int) {
	/*
	   sql := "INSERT INTO frag (victim,attacker,server,fragdate) VALUES (?,?,?,?)"
	   _, err := DB.Exec(sql, server, logtype, logentry, now)

	   	if err != nil {
	   		log.Println(err)
	   		return
	   	}
	*/
}

// Insert client-specific event
func LogEvent(cl *client.Client, event string) {
	s := "INSERT INTO client_log (uuid, event_time, event) VALUES (?,?,?)"
	_, err := DB.Exec(s, cl.UUID, util.GetUnixTimestamp(), event)
	if err != nil {
		log.Println(err)
	}
}

// A player connected, save them in the database
//
// Called from ParseConnect()
func LogPlayer(cl *client.Client, pl *client.Player) {
	s := "INSERT INTO player (server, name, ip, hash, userinfo, connect_time) VALUES (?,?,?,?,?,?)"
	_, err := DB.Exec(
		s,
		cl.UUID,
		pl.Name,
		pl.IP,
		pl.UserInfoHash,
		pl.Userinfo,
		util.GetUnixTimestamp(),
	)
	if err != nil {
		log.Println(err)
	}
}

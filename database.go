package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Open our sqlite database
func DatabaseConnect() *sql.DB {
	db, err := sql.Open("sqlite3", q2a.config.Database)
	if err != nil {
		panic(err)
	}

	if err = db.Ping(); err != nil {
		panic(err)
	}

	return db
}

func GetPlayerIdFromHash(hash string) int {
	sql := "SELECT id FROM player WHERE hash = ? LIMIT 1"
	r, err := db.Query(sql, hash)
	if err != nil {
		log.Println(err)
		return 0
	}
	defer r.Close()

	id := 0
	for r.Next() {
		r.Scan(&id)
	}

	return id
}

func InsertPlayer(p *Player) int64 {
	sql := "INSERT INTO player (hash) VALUES (?)"
	r, err := db.Exec(sql, p.Hash)
	if err != nil {
		log.Println(err)
	}

	id, err := r.LastInsertId()
	if err != nil {
		log.Println(err)
		return 0
	}

	return id
}

/**
 * A player said something, record to use against them later
 */
func LogChat(cl *Client, chat string) {
	sql := "INSERT INTO chat (server, time, chat) VALUES (?,?,?)"
	_, err := db.Exec(sql, cl.ID, GetUnixTimestamp(), chat)
	if err != nil {
		log.Println(err)
		return
	}
}

/**
 * Save frags for stats
 */
func LogFrag(cl *Client, victim int, attacker int) {
	/*
		sql := "INSERT INTO frag (victim,attacker,server,fragdate) VALUES (?,?,?,?)"
		_, err := db.Exec(sql, server, logtype, logentry, now)
		if err != nil {
			log.Println(err)
			return
		}
	*/
}

func LogEventToDatabase(cid int, logtype int, logentry string) {
	now := time.Now().Unix()
	sql := "INSERT INTO logdata (server, msgtype, entry, entrydate) VALUES (?,?,?,?)"
	_, err := db.Exec(sql, cid, logtype, logentry, now)
	if err != nil {
		log.Println(err)
		return
	}
}

// Insert client-specific event
func (cl *Client) LogEvent(event string) {
	s := "INSERT INTO client_log (uuid, log_time, log_entry) VALUES (?,?,?)"
	_, err := db.Exec(s, cl.UUID, GetUnixTimestamp(), event)
	if err != nil {
		log.Println(err)
	}
}

// Insert an system event into the db
func LogSystemEvent(event string) {
	s := "INSERT INTO system_log (log_time, log_entry) VALUES (?,?)"
	_, err := db.Exec(s, GetUnixTimestamp(), event)
	if err != nil {
		log.Println(err)
	}
}

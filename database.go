package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func DatabaseConnect() *sql.DB {
	db, err := sql.Open("sqlite3", config.Database)
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
 * Pull all the servers from the database and load them
 * into a structure
 */
func LoadServers(db *sql.DB) []Client {
	sql := "SELECT id, uuid, owner, name, ip, port, disabled FROM server"
	r, err := db.Query(sql)
	if err != nil {
		panic(err)
	}
	var cls []Client
	var cl Client
	var disabled int
	for r.Next() {
		r.Scan(&cl.ID, &cl.UUID, &cl.Owner, &cl.Name, &cl.IPAddress, &cl.Port, &disabled)
		cl.Enabled = disabled == 0
		cls = append(cls, cl)
	}
	r.Close()

	return cls
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

package main

import (
	"database/sql"
	"log"
	"time"

	//_ "github.com/go-sql-driver/mysql"
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
	r, err := db.Exec(sql, p.hash)
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
func LoadServers(db *sql.DB) []Server {
	sql := "SELECT id, uuid, owner, name, ip, port, disabled FROM server"
	r, err := db.Query(sql)
	if err != nil {
		panic(err)
	}
	var srvs []Server
	var srv Server
	var disabled int
	for r.Next() {
		r.Scan(&srv.ID, &srv.UUID, &srv.Owner, &srv.Name, &srv.ipaddress, &srv.port, &disabled)
		srv.enabled = disabled == 0
		srvs = append(srvs, srv)
	}
	r.Close()

	return srvs
}

/**
 * A player said something, record to use against them later
 */
func LogChat(srv *Server, chat string) {
	sql := "INSERT INTO chat (server, time, chat) VALUES (?,?,?)"
	_, err := db.Exec(sql, srv.ID, GetUnixTimestamp(), chat)
	if err != nil {
		log.Println(err)
		return
	}
}

/**
 * Save frags for stats
 */
func LogFrag(srv *Server, victim int, attacker int) {
	/*
		sql := "INSERT INTO frag (victim,attacker,server,fragdate) VALUES (?,?,?,?)"
		_, err := db.Exec(sql, server, logtype, logentry, now)
		if err != nil {
			log.Println(err)
			return
		}
	*/
}

func LogEventToDatabase(server int, logtype int, logentry string) {
	now := time.Now().Unix()
	sql := "INSERT INTO logdata (server, msgtype, entry, entrydate) VALUES (?,?,?,?)"
	_, err := db.Exec(sql, server, logtype, logentry, now)
	if err != nil {
		log.Println(err)
		return
	}
}

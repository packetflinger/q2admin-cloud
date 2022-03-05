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
		r.Scan(&srv.id, &srv.uuid, &srv.owner, &srv.name, &srv.ipaddress, &srv.port, &disabled)
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
	_, err := db.Exec(sql, srv.id, GetUnixTimestamp(), chat)
	if err != nil {
		log.Println(err)
		return
	}
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

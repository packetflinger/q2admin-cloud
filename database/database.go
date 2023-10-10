package database

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/util"
)

// Open our sqlite database
func DatabaseConnect() *sql.DB {
	db, err := sql.Open("sqlite3", Q2A.config.Database)
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
	r, err := DB.Query(sql, hash)
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
	r, err := DB.Exec(sql, p.UserInfoHash)
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

func LogEventToDatabase(cid int, logtype int, logentry string) {
	now := time.Now().Unix()
	sql := "INSERT INTO logdata (server, msgtype, entry, entrydate) VALUES (?,?,?,?)"
	_, err := DB.Exec(sql, cid, logtype, logentry, now)
	if err != nil {
		log.Println(err)
		return
	}
}

// Insert an system event into the db
func LogSystemEvent(event string) {
	s := "INSERT INTO system_log (log_time, log_entry) VALUES (?,?)"
	_, err := DB.Exec(s, GetUnixTimestamp(), event)
	if err != nil {
		log.Println(err)
	}
}

// A player connected, save them in the database
//
// Called from ParseConnect()
func LogPlayer(cl *client.Client, pl *client.Player, db *sql.DB) {
	s := "INSERT INTO player (server, name, ip, hash, userinfo, connect_time) VALUES (?,?,?,?,?,?)"
	_, err := db.Exec(
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

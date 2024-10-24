package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/util"
)

const (
	schema = `
CREATE TABLE IF NOT EXISTS "player" (
	"id"		INTEGER,
	"server"	TEXT,
	"name"		TEXT,
	"ip"		TEXT,
	"hostname"	TEXT,
	"vpn"		INTEGER,
	"cookie"	TEXT,
	"version"	TEXT,
	"userinfo"	TEXT,
	"time"		INTEGER,
	PRIMARY KEY("id" AUTOINCREMENT)
);`

	insertPlayer = `
INSERT INTO player (server, name, ip, hostname, vpn, cookie, version, userinfo, time) 
VALUES (?,?,?,?,?,?,?,?,?)`
)

// A struct for holding all our DB stuff
type Database struct {
	Handle *sql.DB
}

func (d Database) Begin() (*sql.Tx, error) {
	return d.Handle.Begin()
}

// Add will insert the player into the database
func (d Database) Add(pl *client.Player) error {
	st, err := d.Handle.Prepare(insertPlayer)
	if err != nil {
		return fmt.Errorf("error preparing player insert: %v", err)
	}
	defer st.Close()
	_, err = st.Exec(pl.Client.Name, pl.Name, pl.IP, pl.Hostname, pl.VPN, pl.Cookie, pl.Version, pl.Userinfo, util.GetUnixTimestamp())
	if err != nil {
		return fmt.Errorf("error inserting player data: %v", err)
	}
	return nil
}

// Open will open the database file and return a struct that holds the handle
// to the db. If no database file exists, a new one will be created.
//
// Called from server.Startup()
func Open(filename string) (Database, error) {
	var database Database
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return database, fmt.Errorf("error opening database: %v", err)
	}
	if err = db.Ping(); err != nil {
		return database, fmt.Errorf("error pinging database: %v", err)
	}
	row := db.QueryRow("PRAGMA schema_version")
	var version int
	err = row.Scan(&version)
	if err != nil {
		return database, fmt.Errorf("error scanning db schema: %v", err)
	}
	if version == 0 {
		_, err := db.Exec(schema)
		if err != nil {
			return database, fmt.Errorf("error loading db schema: %v", err)
		}
	}
	database.Handle = db
	return database, nil
}

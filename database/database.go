package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/packetflinger/q2admind/frontend"
	"github.com/packetflinger/q2admind/util"

	_ "github.com/mattn/go-sqlite3"
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

	search = `SELECT * FROM player WHERE
	(name LIKE ? OR ip LIKE ? OR hostname LIKE ? OR userinfo LIKE ?)`
)

// A struct for holding all our DB stuff
type Database struct {
	Handle *sql.DB
}

// A slice of these is returned for a search. Each represents a player record.
type SearchResult struct {
	ID       int
	Server   string
	Name     string
	IP       string
	Hostname string
	VPN      bool
	Cookie   string
	Version  string
	Userinfo string
	Time     int64
	Ago      string
}

func (d Database) Begin() (*sql.Tx, error) {
	return d.Handle.Begin()
}

// AddPlayer will insert the player into the database
func (d Database) AddPlayer(pl *frontend.Player) error {
	if pl == nil {
		return fmt.Errorf("error adding player to db: null player")
	}
	_, err := d.Handle.Exec(
		insertPlayer, pl.Frontend.Name, pl.Name, pl.IP, pl.Hostname, pl.VPN,
		pl.Cookie, pl.Version, pl.Userinfo, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("error inserting player %s[%s]: %v", pl.Name, pl.IP, err)
	}
	return nil
}

// Open will open the database file and return a struct that holds the handle
// to the db. If no database file exists, a new one will be created.
//
// Called from server.Startup()
func Open(filename string) (Database, error) {
	var database Database
	if filename == "" {
		return database, fmt.Errorf("blank database file string")
	}
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

// Search will fetch the rows that match the input pattern.
func (d Database) Search(pattern string) ([]SearchResult, error) {
	var results []SearchResult
	if pattern == "" {
		return results, fmt.Errorf("blank search string")
	}
	if len(pattern) < 3 {
		return nil, fmt.Errorf("error search input needs to be at least 3 characters")
	}
	pattern = "%" + pattern + "%"
	st, err := d.Handle.Prepare(search)
	if err != nil {
		return nil, fmt.Errorf("error preparing statement: %v", err)
	}
	defer st.Close()
	res, err := st.Query(pattern, pattern, pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("error querying for %q: %v", pattern, err)
	}
	defer res.Close()
	for res.Next() {
		var r SearchResult
		err := res.Scan(&r.ID, &r.Server, &r.Name, &r.IP, &r.Hostname, &r.VPN, &r.Cookie, &r.Version, &r.Userinfo, &r.Time)
		if err != nil {
			return nil, fmt.Errorf("error scanning results: %v", err)
		}
		r.Ago = util.TimeAgo(r.Time)
		results = append(results, r)
	}
	return results, nil
}

package database

import (
	"database/sql"
	"fmt"

	// "github.com/packetflinger/q2admind/frontend"
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

	//	insertPlayer = `
	// INSERT INTO player (server, name, ip, hostname, vpn, cookie, version, userinfo, time)
	// VALUES (?,?,?,?,?,?,?,?,?)`

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
	if _, err := db.Exec(groupSchema); err != nil {
		return database, fmt.Errorf("error creating server group tables: %v", err)
	}
	if _, err := db.Exec(statsSchema); err != nil {
		return database, fmt.Errorf("error creating stats tables: %v", err)
	}
	for _, c := range columnAdditions {
		if err := addColumnIfMissing(db, c.table, c.column, c.decl); err != nil {
			return database, err
		}
	}
	database.Handle = db
	return database, nil
}

// groupSchema holds user-defined groups of servers (a frontend can be in any
// number of groups; names are unique per owner) and the users each frontend's
// owner has delegated it to.
const groupSchema = `
CREATE TABLE IF NOT EXISTS "server_group" (
	"id"	INTEGER PRIMARY KEY AUTOINCREMENT,
	"owner"	TEXT NOT NULL,
	"name"	TEXT NOT NULL,
	UNIQUE("owner", "name")
);
CREATE TABLE IF NOT EXISTS "frontend_delegate" (
	"frontend_id"	INTEGER NOT NULL,
	"email"	TEXT NOT NULL,
	PRIMARY KEY("frontend_id", "email")
);
CREATE TABLE IF NOT EXISTS "server_group_member" (
	"group_id"	INTEGER NOT NULL,
	"frontend_id"	INTEGER NOT NULL,
	PRIMARY KEY("group_id", "frontend_id")
);`

// statsSchema holds the samples the maintenance thread takes for the dashboard
// graphs: connected players per server, and kills per weapon per server since
// the previous sample.
const statsSchema = `
CREATE TABLE IF NOT EXISTS "server_sample" (
	"time"	INTEGER NOT NULL,
	"frontend_id"	INTEGER NOT NULL,
	"players"	INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS "server_sample_time" ON "server_sample" ("time");
CREATE TABLE IF NOT EXISTS "frag_sample" (
	"time"	INTEGER NOT NULL,
	"frontend_id"	INTEGER NOT NULL,
	"weapon"	TEXT NOT NULL,
	"kills"	INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS "frag_sample_time" ON "frag_sample" ("time");
CREATE INDEX IF NOT EXISTS "player_time" ON "player" ("time");`

// columnAdditions are small schema changes applied to existing databases when
// they're opened.
var columnAdditions = []struct{ table, column, decl string }{
	{"rule", "description", "TEXT"},
	{"rule", "vpn", "INTEGER DEFAULT 0"},
}

// addColumnIfMissing adds column to table, unless the table doesn't exist or
// already has it. The names come from columnAdditions, never from input.
func addColumnIfMissing(db *sql.DB, table, column, decl string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return fmt.Errorf("reading %s schema: %v", table, err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("reading %s schema: %v", table, err)
		}
		if name == column {
			return nil
		}
		found = true
	}
	if !found { // no such table
		return nil
	}
	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q %s", table, column, decl)); err != nil {
		return fmt.Errorf("adding %s.%s: %v", table, column, err)
	}
	return nil
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

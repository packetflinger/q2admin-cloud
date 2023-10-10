package database

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// Open our sqlite database
func DatabaseConnect(dbfile string) *sql.DB {
	db, err := sql.Open("sqlite3", dbfile)
	if err != nil {
		panic(err)
	}

	if err = db.Ping(); err != nil {
		panic(err)
	}

	return db
}

package database

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
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

package main

import "log"

//
// Someone deleted a managed server via the web interface.
// This should mean:
// - remove from database, including foreign key constraints
// - close any open connections to this server
// - remove from active server slice in memory
//
func RemoveServer(uuid string) bool {
	srv, err := findserver(uuid)
	if err == nil {
		// mark in-ram server object as disabled to prevent reconnects
		srv.Enabled = false

		// close any connections?
	}

	tr, err := db.Begin()
	if err != nil {
		log.Println(err)
		return false
	}

	sql := "DELETE FROM server WHERE id = ?"
	_, err = tr.Exec(sql, srv.ID)
	if err != nil {
		log.Println(err)
		tr.Rollback()
		return false
	}

	// log data?
	// chat data?

	tr.Commit()
	return true
}

package main

import "log"

//
// Resync servers struct with database
// Should get called when servers are added/removed
// via the web interface
//
func RehashServers() []Server {
	//newservers := []Server{}
	sql := "SELECT id, uuid, owner, name, ip, port, disabled FROM server"
	r, err := db.Query(sql)
	if err != nil {
		log.Println(err)
		return servers // db error, return current struct
	}

	var srvs []Server
	var srv Server
	var disabled int
	//current := Server{}
	for r.Next() {
		r.Scan(&srv.ID, &srv.UUID, &srv.Owner, &srv.Name, &srv.IPAddress, &srv.Port, &disabled)
		srv.Enabled = disabled == 0

		current, err := findserver(srv.UUID)
		if err != nil { // server in db, but not memory
			srvs = append(srvs, srv)
		} else {
			srvs = append(srvs, *current)
		}
	}
	r.Close()

	return srvs
}

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

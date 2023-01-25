package main

import (
	"log"
	"net/http"
	"time"

	uuid "github.com/google/uuid"
)

func GenerateUUID() string {
	return uuid.NewString()
}

//
// Remove any active sessions
//
func AuthLogout(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	// no current session
	if err != nil {
		return
	}

	// remove current session
	user.Session = UserSession{}

	// remove the client's cookie
	expire := time.Now()
	cookie := http.Cookie{Name: SessionName, Value: "", Expires: expire}
	http.SetCookie(w, &cookie)
}

//
// Resync servers struct with database
// Should get called when servers are added/removed
// via the web interface
//
func RehashServers() []Client {
	sql := "SELECT id, uuid, owner, name, ip, port, disabled FROM server"
	r, err := db.Query(sql)
	if err != nil {
		log.Println(err)
		return q2a.clients // db error, return current struct
	}

	var cls []Client
	var cl Client
	var disabled int
	for r.Next() {
		r.Scan(&cl.ID, &cl.UUID, &cl.Owner, &cl.Name, &cl.IPAddress, &cl.Port, &disabled)
		cl.Enabled = disabled == 0

		current, err := FindClient(cl.UUID)
		if err != nil { // server in db, but not memory
			cls = append(cls, cl)
		} else {
			cls = append(cls, *current)
		}
	}
	r.Close()

	return cls
}

//
// Someone deleted a managed server via the web interface.
// This should mean:
// - remove from database, including foreign key constraints
// - close any open connections to this server
// - remove from active server slice in memory
//
func RemoveServer(uuid string) bool {
	cl, err := FindClient(uuid)
	if err == nil {
		// mark in-ram server object as disabled to prevent reconnects
		cl.Enabled = false

		// close any connections?
	}

	tr, err := db.Begin()
	if err != nil {
		log.Println(err)
		return false
	}

	sql := "DELETE FROM server WHERE id = ?"
	_, err = tr.Exec(sql, cl.ID)
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

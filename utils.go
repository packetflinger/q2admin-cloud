package main

import (
	"fmt"
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

func RedirectToSignon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, routes.AuthLogin, http.StatusFound) // 302
}

// TimeAgo gives you a string of how long ago something was
// based on a unix timestamp.
// Examples:
//   just now
//   30s ago
//   5m ago
//   2h ago
//   3d ago
//   8w ago
//   2mo ago
//   3yr ago
func TimeAgo(ts int64) string {
	elapsed := GetUnixTimestamp() - ts
	if elapsed < 0 {
		return "soon"
	}
	if elapsed < 5 {
		return "just now"
	}
	if elapsed < 60 {
		return fmt.Sprintf("%ds ago", elapsed)
	}
	if elapsed < 3600 {
		return fmt.Sprintf("%dm ago", elapsed/60)
	}
	if elapsed < 86400 {
		return fmt.Sprintf("%dh ago", elapsed/3600)
	}
	if elapsed < 86400*7 {
		return fmt.Sprintf("%dd ago", elapsed/86400)
	}
	if elapsed < 86400*30 {
		return fmt.Sprintf("%dw ago", elapsed/(86400*7))
	}
	if elapsed < 86400*30*52 {
		return fmt.Sprintf("%dy ago", elapsed/(86400*30))
	}
	return "forever ago"
}

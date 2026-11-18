// Website search pages for players and servers.

package backend

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	pb "github.com/packetflinger/q2admind/proto"
)

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{SessionUser: user, SearchQuery: strings.TrimSpace(r.URL.Query().Get("q"))}
	data.Head.Title = "Search | CloudAdmin"
	data.ServerNames = myServerNames(user)
	if data.SearchQuery != "" {
		var servers []string
		for uuid := range data.ServerNames { // only the user's own servers
			servers = append(servers, uuid)
		}
		data.SearchResults, err = SearchPlayers(r.Context(), data.SearchQuery, servers)
		if err != nil {
			log.Printf("error searching %q: %v\n", data.SearchQuery, err)
			http.Error(w, "500 - search failed", http.StatusInternalServerError)
			return
		}
	}
	tmpl, err := parsePage("search.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "search", data); err != nil {
		log.Println(err)
	}
}

// myServerNames maps the UUIDs of the servers user owns to their names.
func myServerNames(user *pb.User) map[string]string {
	names := map[string]string{}
	for _, fe := range FrontendsByIdentity(user.GetEmail()) {
		fe.Mu.Lock()
		names[fe.UUID] = fe.Name
		fe.Mu.Unlock()
	}
	return names
}

func ServerSearchHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	f := frontendFor(w, r, user, false)
	if f == nil {
		return
	}
	data := PageResponse{SessionUser: user, Frontend: f, SearchQuery: strings.TrimSpace(r.URL.Query().Get("q"))}
	data.Head.Title = "Search | CloudAdmin"
	data.NavHighlight.Servers = "active"
	if data.SearchQuery != "" {
		f.Mu.Lock()
		server := f.UUID
		f.Mu.Unlock()
		data.SearchResults, err = SearchPlayers(r.Context(), data.SearchQuery, []string{server})
		if err != nil {
			log.Printf("error searching %q: %v\n", data.SearchQuery, err)
			http.Error(w, "500 - search failed", http.StatusInternalServerError)
			return
		}
	}
	tmpl, err := parsePage("search.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "search", data, f); err != nil {
		log.Println(err)
	}
}

func PlayerSearchViewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	lu, err := base64.StdEncoding.DecodeString(mux.Vars(r)["lookup"])
	name, when, ok := strings.Cut(string(lu), "\\")
	if err != nil || !ok {
		http.Error(w, "400 - invalid player lookup", http.StatusBadRequest)
		return
	}
	qry := `
	SELECT p.id, p.name, p.ip, p.hostname, p.vpn, p.cookie, p.version, p.userinfo, p.time, f.uuid
	FROM player AS p
	JOIN frontend AS f ON f.name = p.server -- player.server holds the frontend name
	WHERE p.name = ? AND p.time = ?
	LIMIT 1`
	var p PlayerDatabaseInfo
	err = db.Handle.QueryRow(qry, name, when).Scan(&p.Id, &p.Name, &p.IP, &p.Hostname, &p.Vpn, &p.Cookie, &p.Version, &p.Userinfo, &p.ConnectTime, &p.ServerUUID)
	data := PageResponse{SessionUser: user, ServerNames: myServerNames(user)}
	if _, mine := data.ServerNames[p.ServerUUID]; err != nil || !mine {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Printf("error looking up player %q: %v\n", name, err)
		}
		http.NotFound(w, r)
		return
	}
	data.PlayerDBInfo = p
	data.Head.Title = p.Name + " | CloudAdmin"
	tmpl, err := parsePage("player.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "player", data); err != nil {
		log.Println(err)
	}
}

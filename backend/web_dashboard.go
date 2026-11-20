// The website dashboard and server list.

package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"

	"github.com/gorilla/mux"

	pb "github.com/packetflinger/q2admind/proto"
)

// Dashboard handler
//
// This is the main landing page after authenticating
func WebsiteHandlerDashboard(w http.ResponseWriter, r *http.Request) {
	u, err := GetSessionUser(r)
	if err != nil {
		http.Redirect(w, r, Routes.AuthLogin, http.StatusFound) // 302
		return
	}

	page, err := dashboardPage(u, r.URL.Query().Get("span"))
	if err != nil {
		log.Println(err)
		fmt.Fprintln(w, "error 500")
		return
	}

	tmpl, e := parsePage("dashboard.tmpl")

	if e != nil {
		log.Println(e)
	} else {
		err = executeLocked(w, tmpl, "dashboard", page, page.Frontends...)
		if err != nil {
			log.Println(err)
		}
	}
}

func dashboardPage(user *pb.User, span string) (PageResponse, error) {
	var out PageResponse
	if user == nil {
		return out, fmt.Errorf("null user building dashboard page")
	}
	out.SessionUser = user
	out.Head.Title = "Dashboard | CloudAdmin"
	out.Frontends = FrontendsByIdentity(user.GetEmail())
	out.NavHighlight.Dashboard = "active"
	stats, err := loadStats(out.Frontends, span, true) // one row per player
	out.Stats = stats
	return out, err
}

func WebsiteAPIGetConnectedServers(w http.ResponseWriter, r *http.Request) {
	var activeservers []ActiveServer
	for _, s := range be.Frontends() {
		s.Mu.Lock()
		if s.Connected {
			srv := ActiveServer{UUID: s.UUID, Name: s.Name, Playercount: len(s.Players)}
			activeservers = append(activeservers, srv)
		}
		s.Mu.Unlock()
	}

	j, e := json.Marshal(activeservers)
	if e != nil {
		fmt.Println(e)
		fmt.Fprintf(w, "{}")
		return
	}

	fmt.Fprintf(w, "%s", string(j))
}

// Handler for the /my-servers page
func ServersHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	fes := FrontendsByIdentity(user.Email)

	data := PageResponse{}
	data.Head.Title = "My Servers | Q2Admin CloudAdmin"
	data.Head.Keywords = ""
	data.Title = "My Servers"
	data.SessionUser = user
	data.Frontends = fes
	data.FrontendCount = len(fes)

	data.NavHighlight.Servers = "active"

	tmpl, e := parsePage("servers.tmpl")

	if e != nil {
		log.Println(e)
	} else {
		err = executeLocked(w, tmpl, "my-servers", data, fes...)
		if err != nil {
			log.Println(err)
		}
	}
}

// RecentPlayerLimits are the choices for how many visits /recent-players
// shows, and the default.
var (
	RecentPlayerLimits = []int{10, 20, 50, 100, 200}
	recentPlayerLimit  = 20
)

// RecentPlayerPage is one player's visits to the user's servers, newest first.
type RecentPlayerPage struct {
	Name     string
	Sessions []Session
	Limit    int   // how many visits to show
	Limits   []int // the choices for Limit
	Signals  []SignalEvent
}

// RecentPlayerHandler lists the visits of the player the dashboard's recent
// players table links to, across all the user's servers.
func RecentPlayerHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	limit := recentPlayerLimit
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && slices.Contains(RecentPlayerLimits, n) {
		limit = n
	}
	name := mux.Vars(r)["name"]
	fes := FrontendsByIdentity(user.GetEmail())
	sessions, err := loadSessions(fes, 0, name, limit, false)
	if err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to load the player's visits", http.StatusInternalServerError)
		return
	}
	signals, err := loadSignals(fes, name, signalsShown)
	if err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to load the player's signals", http.StatusInternalServerError)
		return
	}
	data := PageResponse{SessionUser: user, RecentPlayer: &RecentPlayerPage{Name: name, Sessions: sessions, Limit: limit, Limits: RecentPlayerLimits, Signals: signals}}
	data.Head.Title = name + " | CloudAdmin"
	data.NavHighlight.Dashboard = "active"
	tmpl, err := parsePage("recent-player.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "recent-player", data); err != nil {
		log.Println(err)
	}
}

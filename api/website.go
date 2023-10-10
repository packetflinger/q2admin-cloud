package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/packetflinger/q2admind/client"
)

const (
	SessionName = "q2asess" // website cookie name
)

var (
	Website = WebInterface{}
)

const (
	FeedChat = iota
	FeedFrag
	FeedJoinPart
	FeedBan
	FeedMute
)

type WebpageMessage struct {
	Quantity int
	Icon     string
	Name     string
	Content  string
	Timing   string
}
type WebpageNotification struct {
	Icon    string
	Title   string
	Content string
	Timing  string
}
type WebpageData struct {
	Title           string
	HeaderTitle     string
	Notification    []WebpageNotification
	Message         []WebpageMessage
	SessionUser     *User
	Gameservers     []*client.Client
	GameserverCount int
	Client          *client.Client
	NavHighlight    struct {
		Dashboard string
		Servers   string
		Groups    string
	}
}

type ActiveServer struct {
	UUID        string
	Name        string
	Playercount int
}

type WebUser struct {
	ID          int
	UUID        string
	Email       string
	ServerCount int
	Admin       bool
}

type DashboardPage struct {
	WebUser      *User
	MyServers    []client.Client
	OtherServers []client.Client
}

type ServerPage struct {
	WebUser  *User
	MyServer client.Client
}

// Represents the website
type WebInterface struct {
	Creds []Credentials
}

// needed for upgrading the websockets
var WSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1500,
	WriteBufferSize: 1500,
}

// Gets a pointer to the user associated with the current
// session. If no session exists, error will be set.
// Session validit is also checked: expiration, user mismatch
//
// Called at the start of each website request
func GetSessionUser(r *http.Request) (*User, error) {
	var user *User
	var cookie *http.Cookie
	var e error
	niluser := &User{}

	if cookie, e = r.Cookie(SessionName); e != nil {
		return niluser, e
	}

	if user, e = ValidateSession(cookie.Value); e != nil {
		return niluser, e
	}

	return user, nil
}

func GetUser(id int) WebUser {
	niluser := &WebUser{}
	sql := "SELECT id, uuid, email, server_count, admin FROM user WHERE id = ? LIMIT 1"
	r, e := db.Query(sql, id)
	if e != nil {
		log.Println(e)
		return *niluser
	}

	var user WebUser
	for r.Next() {
		r.Scan(&user.ID, &user.UUID, &user.Email, &user.ServerCount, &user.Admin)
		r.Close()
		return user
	}

	return *niluser
}

// Make a new session for a user
func CreateSession() UserSession {
	sess := UserSession{
		ID:      GenerateUUID(),
		Created: GetUnixTimestamp(),
		Expires: GetUnixTimestamp() + (86400 * 2), // 2 days from now
	}

	return sess
}

// Make sure the session presented is valid.
// 1. Current date is after the session creation date
// 2. Current date is before the session expiration
func ValidateSession(sess string) (*User, error) {
	/*
		for i := range q2a.Users {
			u := q2a.Users[i]
			if u.Session.ID == sess {
				now := GetUnixTimestamp()
				if now > u.Session.Created && now < u.Session.Expires {
					return &u, nil
				}
			}
		}
	*/
	return &User{}, errors.New("invalid session")
}

// Load everything needed to start the web interface
func RunHTTPServer() {
	// load our OAuth2 stuff
	cr, err := ReadOAuthCredsFromDisk(q2a.config.GetAuthFile())
	if err != nil {
		log.Println(err)
		os.Exit(0)
	}
	Website.Creds = cr

	port := fmt.Sprintf("0.0.0.0:%d", q2a.config.GetApiPort())
	r := LoadWebsiteRoutes()

	httpsrv := &http.Server{
		Handler:      r,
		Addr:         port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Printf("Listening for web requests on %s\n", port)
	log.Fatal(httpsrv.ListenAndServe())
}

func WebsiteHandlerDashboard(w http.ResponseWriter, r *http.Request) {
	u, err := GetSessionUser(r)
	if err != nil {
		http.Redirect(w, r, Routes.AuthLogin, http.StatusFound) // 302
		return
	}

	data := WebpageData{
		Title:       "My Servers | Q2Admin CloudAdmin",
		HeaderTitle: "My Servers",
		SessionUser: u,
	}
	data.NavHighlight.Dashboard = "active"

	tmpl, e := template.ParseFiles(
		"website/templates/home.tmpl",
		"website/templates/header-main.tmpl",
		"website/templates/footer.tmpl",
	)

	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "home", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func WebsiteHandlerServerView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["ServerUUID"]
	name := vars["ServerName"]
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	cl, err := FindClient(uuid)
	if err != nil {
		log.Println("invalid server id:", uuid)
		return
	}

	data := WebpageData{
		Title:       name + " management | Q2Admin CloudAdmin",
		HeaderTitle: name,
		SessionUser: user,
		Client:      cl,
	}
	data.NavHighlight.Servers = "active"

	tmpl, e := template.ParseFiles(
		"website/templates/header-main.tmpl",
		"website/templates/server-view.tmpl",
		"website/templates/footer.tmpl",
	)

	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "server-view", data)
		if err != nil {
			log.Println(err)
		}
	}
}

// the "index" handler
func WebsiteHandlerIndex(w http.ResponseWriter, r *http.Request) {
	_, e := GetSessionUser(r)
	if e != nil {
		http.Redirect(w, r, Routes.AuthLogin, http.StatusFound) // 302
		return
	}

	http.Redirect(w, r, Routes.Dashboard, http.StatusFound) // 302
}

// Display signin page
func WebsiteHandlerSignin(w http.ResponseWriter, r *http.Request) {
	tmpl, e := template.ParseFiles("website/templates/sign-in.tmpl")
	for i := range Website.Creds {
		Website.Creds[i].URL = BuildAuthURL(Website.Creds[i], i)
	}

	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, Website.Creds)
	}
}

func WebsiteAPIGetConnectedServers(w http.ResponseWriter, r *http.Request) {
	var activeservers []ActiveServer
	for _, s := range q2a.clients {
		if s.Connected {
			srv := ActiveServer{UUID: s.UUID, Name: s.Name, Playercount: len(s.Players)}
			activeservers = append(activeservers, srv)
		}
	}

	j, e := json.Marshal(activeservers)
	if e != nil {
		fmt.Println(e)
		fmt.Fprintf(w, "{}")
		return
	}

	fmt.Fprintf(w, string(j))
}

func WebAddServer(w http.ResponseWriter, r *http.Request) {
	/*
		user := GetSessionUser(r)
		r.ParseForm()
		name := r.Form.Get("servername")
		ip := r.Form.Get("ipaddr")
		port, err := strconv.Atoi(r.Form.Get("port"))
		if err != nil {
			return
		}
		uuid := uuid.New().String()
		owner := user.ID
		code := "abc123"

		sql := "INSERT INTO server (uuid, owner, name, ip, port, disabled, verified, verify_code) VALUES (?,?,?,?,?,0,0,?)"
		_, err = db.Exec(sql, uuid, owner, name, ip, port, code)
		if err != nil {
			fmt.Println(err)
			return
		}

		q2a.clients = RehashServers()

		http.Redirect(w, r, routes.Dashboard, http.StatusFound) // 302
	*/
}

// Handler to delete a user's server
func WebDelServer(w http.ResponseWriter, r *http.Request) {
	//user := GetSessionUser(r)
	vars := mux.Vars(r)

	uuid_to_delete := vars["id"]
	srv, err := FindClient(uuid_to_delete)
	if err != nil {
		log.Println(err)
		return
	}

	// check ownership
	//if srv.Owner != user.ID {
	//	log.Printf("%s unsuccessfuly tried to delete %s, non-ownership", user.Email, srv.Name)
	//	return
	//}

	RemoveServer(srv.UUID)
	q2a.clients = RehashServers()
	http.Redirect(w, r, Routes.Dashboard, http.StatusFound)
}

// Log a user out
func WebSignout(w http.ResponseWriter, r *http.Request) {
	AuthLogout(w, r)
	http.Redirect(w, r, Routes.Index, http.StatusFound)
}

// Websocket handler for sending chat message to web clients
func WebFeed(w http.ResponseWriter, r *http.Request) {
	/*
		vars := mux.Vars(r)
		uuid := vars["ServerUUID"]
		page := ServerPage{}
		page.User = GetSessionUser(r)
		srv, err := FindClient(uuid)
		if err != nil {
			log.Println(err)
			return
		}

		WSUpgrader.CheckOrigin = func(r *http.Request) bool {
			// check for auth here
			return true // everyone can connect
		}

		ws, err := WSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			err = nil
		}

		srv.WebSockets = append(srv.WebSockets, ws)

		log.Println("Chat Websocket connected")
	*/
}

func WebFeedInput(w http.ResponseWriter, r *http.Request) {
	/*
		vars := mux.Vars(r)
		uuid := vars["ServerUUID"]
		user := GetSessionUser(r)
		srv, err := FindClient(uuid)
		if err != nil {
			log.Println(err)
			return
		}

		// make sure user is allowed to give commands to srv
		// change this
		if user.ID > 0 {

		}

		//input64 := r.PostForm["input"]
		input64 := r.URL.Query().Get("input")
		input, err := base64.StdEncoding.DecodeString(input64)
		if err != nil {
			log.Println(err)
			return
		}

		preamble := "[" + user.Email + "] "
		srv.SendToWebsiteFeed(preamble+string(input), FeedChat)
	*/
}

func GroupsHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	data := WebpageData{
		Title:       "My Groups | Q2Admin CloudAdmin",
		HeaderTitle: "My Groups",
		SessionUser: user,
	}
	data.NavHighlight.Groups = "active"

	tmpl, e := template.ParseFiles(
		"website/templates/header-main.tmpl",
		"website/templates/my-groups.tmpl",
		"website/templates/footer.tmpl",
	)
	if e != nil {
		log.Println(e)
	} else {
		err := tmpl.ExecuteTemplate(w, "my-groups", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func ServersHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	data := WebpageData{
		Title:       "My Servers | Q2Admin CloudAdmin",
		HeaderTitle: "My Servers",
		SessionUser: user,
	}

	data.NavHighlight.Servers = "active"

	// build server list
	svs := []*Client{}
	for i := range q2a.clients {
		if q2a.clients[i].Owner == user.ID {
			svs = append(svs, &q2a.clients[i])
		}
	}
	data.GameserverCount = len(svs)
	data.Gameservers = svs

	tmpl, e := template.ParseFiles(
		"website/templates/header-main.tmpl",
		"website/templates/my-servers.tmpl",
		"website/templates/footer.tmpl",
		"website/templates/server_templates.tmpl",
	)

	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "my-servers", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func PrivacyHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	data := WebpageData{
		Title:       "Privacy Policy | Q2Admin CloudAdmin",
		HeaderTitle: "Privacy Policy",
		SessionUser: user,
	}

	tmpl, e := template.ParseFiles(
		"website/templates/header-main.tmpl",
		"website/templates/privacy-policy.tmpl",
		"website/templates/footer.tmpl",
	)

	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "privacy-policy", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func TermsHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	data := WebpageData{
		Title:       "Terms of Use | Q2Admin CloudAdmin",
		HeaderTitle: "Terms of Use",
		SessionUser: user,
	}

	tmpl, e := template.ParseFiles(
		"website/templates/header-main.tmpl",
		"website/templates/terms-of-use.tmpl",
		"website/templates/footer.tmpl",
	)

	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "terms-of-use", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func RedirectToSignon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, Routes.AuthLogin, http.StatusFound) // 302
}

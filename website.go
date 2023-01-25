package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	FeedChat = iota
	FeedFrag
	FeedJoinPart
	FeedBan
	FeedMute
)

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
	User         WebUser
	MyServers    []Client
	OtherServers []Client
}

type ServerPage struct {
	User     WebUser
	MyServer Client
}

// needed for upgrading the websockets
var WSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1500,
	WriteBufferSize: 1500,
}

/**
 * Checks if user has an existing session and validates it
 */
func GetSessionUser(r *http.Request) WebUser {
	var userid int
	var cookie *http.Cookie
	var e error
	niluser := &WebUser{}

	if cookie, e = r.Cookie(SessionName); e != nil {
		return *niluser
	}

	if userid, e = ValidateSession(cookie.Value); e != nil {
		return *niluser
	}

	return GetUser(userid)
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

/**
 * User just successfully authed, insert a new session
 */
func CreateSession(user int) string {
	sessionid := uuid.New().String()
	expires := GetUnixTimestamp() + (86400 * 2) // two day from now

	sql := "INSERT INTO websession (session, user, expiration) VALUES (?, ?, ?)"
	_, err := db.Exec(sql, sessionid, user, expires)
	if err != nil {
		log.Println(err)
	}

	sql = "UPDATE user SET last_login = ? WHERE id = ? LIMIT 1"
	_, err = db.Exec(sql, GetUnixTimestamp(), user)
	if err != nil {
		log.Println(err)
	}

	return sessionid
}

func ValidateSession(sess string) (int, error) {
	var UserID int
	sql := "SELECT user FROM websession WHERE session = ? AND expiration >= ? LIMIT 1"
	if r, e := db.Query(sql, sess, GetUnixTimestamp()); e == nil {
		r.Next()
		r.Scan(&UserID)
		r.Close()
		return UserID, nil
	} else {
		return 0, errors.New(e.Error())
	}
}

func RunHTTPServer() {
	port := fmt.Sprintf("0.0.0.0:%d", q2a.config.APIPort)

	r := LoadWebsiteRoutes()

	log.Printf("Listening for web requests on %s\n", port)

	httpsrv := &http.Server{
		Handler:      r,
		Addr:         port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(httpsrv.ListenAndServe())
}

func WebsiteHandlerDashboard(w http.ResponseWriter, r *http.Request) {

	page := DashboardPage{}

	page.User = GetSessionUser(r)
	if page.User.ID == 0 {
		http.Redirect(w, r, "/signin", http.StatusFound) // 302
	}

	sql := "SELECT uuid, name FROM server WHERE owner = ? ORDER BY name ASC"
	rows, err := db.Query(sql, page.User.ID)
	if err != nil {
		log.Println(err)
		return
	}

	for rows.Next() {
		s := Client{}
		err = rows.Scan(&s.UUID, &s.Name)
		if err != nil {
			log.Println(err)
			return
		}
		page.MyServers = append(page.MyServers, s)
	}

	tmpl, e := template.ParseFiles("website-templates/dashboard.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, page)
	}
}

func WebsiteHandlerServerView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["ServerUUID"]
	page := ServerPage{}
	page.User = GetSessionUser(r)
	MyServer, err := FindClient(uuid)
	if err != nil {
		log.Println(err)
		return
	}
	page.MyServer = *MyServer

	tmpl, e := template.ParseFiles("website-templates/dashboard-server-view.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, page)
	}
}

//
// the "index" handler
//
func WebsiteHandlerIndex(w http.ResponseWriter, r *http.Request) {
	user := GetSessionUser(r)
	if user.ID != 0 {
		http.Redirect(w, r, routes.Dashboard, http.StatusFound) // 302
		return
	}

	http.Redirect(w, r, routes.AuthLogin, http.StatusFound) // 302
}

//
// Handle logins
//
func WebsiteHandlerSignin(w http.ResponseWriter, r *http.Request) {
	// login form submitted, process the login
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			log.Println(err)
		}

		email := r.FormValue("email")

		// lookup the user's ID
		var UserID int
		sql := "SELECT id FROM user WHERE email = ? LIMIT 1"
		rs, e := db.Query(sql, email)
		if e == nil {
			rs.Next()
			rs.Scan(&UserID)
			rs.Close()
			sess := CreateSession(UserID)
			cookieexpire := time.Now().AddDate(0, 0, 2) // years, months, days
			cookie := http.Cookie{Name: SessionName, Value: sess, Expires: cookieexpire}
			http.SetCookie(w, &cookie)
		}

		http.Redirect(w, r, routes.Dashboard, http.StatusFound) // 302
		return
	}

	// ...or show the form
	tmpl, e := template.ParseFiles("website/templates/sign-in.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, nil)
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
}

//
// Handler to delete a user's server
//
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
	http.Redirect(w, r, routes.Dashboard, http.StatusFound)
}

//
// Log a user out
//
func WebSignout(w http.ResponseWriter, r *http.Request) {
	AuthLogout(w, r)
	http.Redirect(w, r, routes.Index, http.StatusFound)
}

//
// Websocket handler for sending chat message to web clients
//
func WebFeed(w http.ResponseWriter, r *http.Request) {
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
}

//
//
//
func WebFeedInput(w http.ResponseWriter, r *http.Request) {
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
}

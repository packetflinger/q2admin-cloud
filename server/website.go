package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/client"
	pb "github.com/packetflinger/q2admind/proto"
	"github.com/packetflinger/q2admind/util"
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

// a
type SessionUser struct {
}

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
	SessionUser     *pb.User
	Gameservers     []client.Client
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
	WebUser      *api.User
	MyServers    []client.Client
	OtherServers []client.Client
}

type ServerPage struct {
	WebUser  *pb.User
	MyServer client.Client
}

// Represents the website
type WebInterface struct {
	Creds []*pb.OAuth
	Auths []AuthProvider
}

type AuthProvider struct {
	URL     string
	Icon    string
	Alt     string
	Enabled bool
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
func GetSessionUser(r *http.Request) (*pb.User, error) {
	var user *pb.User
	var cookie *http.Cookie
	var e error

	if cookie, e = r.Cookie(SessionName); e != nil {
		return nil, e
	}

	if user, e = ValidateSession(cookie.Value); e != nil {
		return nil, e
	}

	return user, nil
}

/*
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
*/

// Make a new session for a user
func CreateSession() *pb.Session {
	sess := pb.Session{
		Id:         util.GenerateUUID(),
		Creation:   util.GetUnixTimestamp(),
		Expiration: util.GetUnixTimestamp() + (86400 * 2), // 2 days from now
	}
	return &sess
}

// Make sure the session presented is valid.
// 1. Current date is after the session creation date
// 2. Current date is before the session expiration
func ValidateSession(sess string) (*pb.User, error) {
	for i := range srv.users {
		u := srv.users[i]
		if u.GetSession().GetId() == sess {
			now := util.GetUnixTimestamp()
			if now >= u.GetSession().GetCreation() && now < u.GetSession().GetExpiration() {
				return u, nil
			}
		}
	}
	return &pb.User{}, errors.New("invalid session")
}

// Load everything needed to start the web interface
func RunHTTPServer(ip string, port int, creds []*pb.OAuth) {
	Website.Creds = creds

	listen := fmt.Sprintf("%s:%d", ip, port)
	r := LoadWebsiteRoutes()

	httpsrv := &http.Server{
		Handler:      r,
		Addr:         listen,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Printf("Listening for web requests on http://%s\n", listen)
	log.Fatal(httpsrv.ListenAndServe())
}

// Dashboard handler
//
// This is the main landing page after authenticating
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
		path.Join(srv.config.GetWebRoot(), "templates", "home.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "header-main.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "footer.tmpl"),
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

// Displays info page for a particular client
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
		path.Join(srv.config.GetWebRoot(), "templates", "header-main.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "server-view.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "footer.tmpl"),
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
		RedirectToSignon(w, r)
		return
	}

	http.Redirect(w, r, Routes.Dashboard, http.StatusSeeOther) // 303
}

// Display signin page
func WebsiteHandlerSignin(w http.ResponseWriter, r *http.Request) {
	infile := path.Join(srv.config.GetWebRoot(), "templates", "sign-in.tmpl")
	tmpl, e := template.ParseFiles(infile)
	auths := []AuthProvider{}
	for i := range Website.Creds {
		a := AuthProvider{
			URL:     BuildAuthURL(Website.Creds[i], i),
			Icon:    Website.Creds[i].GetImagePath(),
			Alt:     Website.Creds[i].GetAlternateText(),
			Enabled: !Website.Creds[i].GetDisabled(),
		}
		auths = append(auths, a)
	}

	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, auths)
	}
}

func WebsiteAPIGetConnectedServers(w http.ResponseWriter, r *http.Request) {
	var activeservers []ActiveServer
	for _, s := range srv.clients {
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

	fmt.Fprintf(w, "%s", string(j))
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
	/*
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
	*/
}

// Remove any active sessions
func AuthLogout(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	// no current session
	if err != nil {
		return
	}

	// remove current session
	user.Session = &pb.Session{}

	// remove the client's cookie
	expire := time.Now()
	cookie := http.Cookie{Name: SessionName, Value: "", Expires: expire}
	http.SetCookie(w, &cookie)
}

// Log a user out
func WebSignout(w http.ResponseWriter, r *http.Request) {
	AuthLogout(w, r)
	http.Redirect(w, r, Routes.Index, http.StatusFound)
}

// Websocket handler for sending chat message to web clients
func WebFeed(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["ServerUUID"]
	page := ServerPage{}
	usr, err := GetSessionUser(r)
	if err != nil {
		log.Println(err)
		return
	}
	page.WebUser = usr
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

func WebFeedInput(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["ServerUUID"]
	user, _ := GetSessionUser(r)
	srv, err := FindClient(uuid)
	if err != nil {
		log.Println(err)
		return
	}

	// make sure user is allowed to give commands to srv
	// change this

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
		path.Join(srv.config.GetWebRoot(), "templates", "header-main.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "my-groups.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "footer.tmpl"),
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

// Handler for the /my-servers page
func ServersHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	cls := ClientsByIdentity(user.Email)
	data := WebpageData{
		Title:           "My Servers | Q2Admin CloudAdmin",
		HeaderTitle:     "My Servers",
		SessionUser:     user,
		Gameservers:     cls,
		GameserverCount: len(cls),
	}

	data.NavHighlight.Servers = "active"

	tmpl, e := template.ParseFiles(
		path.Join(srv.config.GetWebRoot(), "templates", "header-main.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "my-servers.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "footer.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "server_templates.tmpl"),
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
		path.Join(srv.config.GetWebRoot(), "templates", "header-main.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "privacy-policy.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "footer.tmpl"),
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
		path.Join(srv.config.GetWebRoot(), "templates", "header-main.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "terms-of-use.tmpl"),
		path.Join(srv.config.GetWebRoot(), "templates", "footer.tmpl"),
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
	http.Redirect(w, r, Routes.AuthLogin, http.StatusSeeOther) // 303
}

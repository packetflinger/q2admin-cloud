package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

const (
	WebCookieName = "q2asess"
)

var (
	Website = WebInterface{}
	funcMap = template.FuncMap{
		"yesno": boolToYesNo,
	}
)

// PageResponse holds all the possible data to render the pages fort he site.
// This structure is used for every page
type PageResponse struct {
	Head struct {
		Author      string
		Description string
		Keywords    string
		Title       string
	}
	Title         string
	HeaderTitle   string
	SessionUser   *pb.User
	Frontends     []*frontend.Frontend
	FrontendCount int
	Frontend      *frontend.Frontend
	NavHighlight  struct {
		Dashboard string
		Servers   string
		Groups    string
	}
	AuthProviders []AuthProvider
}

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
	Title             string
	HeaderTitle       string
	Notification      []WebpageNotification
	NotificationCount int
	Message           []WebpageMessage
	MessageCount      int
	SessionUser       *pb.User
	Gameservers       []frontend.Frontend
	GameserverCount   int
	Frontend          *frontend.Frontend
	NavHighlight      struct {
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
	MyServers    []frontend.Frontend
	OtherServers []frontend.Frontend
}

type ServerPage struct {
	WebUser  *pb.User
	MyServer frontend.Frontend
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

// Gets a pointer to the user associated with the current session. If no
// session exists, error will be set. Session validit is also checked:
// expiration, user mismatch.
//
// Called at the start of each website request
func GetSessionUser(r *http.Request) (*pb.User, error) {
	var user *pb.User
	var cookie *http.Cookie
	var e error

	if cookie, e = r.Cookie(WebCookieName); e != nil {
		return nil, e
	}
	if user, e = ValidateSession(cookie.Value); e != nil {
		return nil, e
	}
	return user, nil
}

// Make a new session for a user
func CreateSession() *pb.Session {
	sess := pb.Session{
		Id:         uuid.NewString(),
		Creation:   time.Now().Unix(),
		Expiration: time.Now().Unix() + (86400 * 2), // 2 days from now
	}
	return &sess
}

// Make sure the session presented is valid.
// 1. Current date is after the session creation date
// 2. Current date is before the session expiration
func ValidateSession(sess string) (*pb.User, error) {
	for i := range be.users {
		u := be.users[i]
		if u.GetSession().GetId() == sess {
			now := time.Now().Unix()
			if now >= u.GetSession().GetCreation() && now < u.GetSession().GetExpiration() {
				return u, nil
			}
		}
	}
	return &pb.User{}, errors.New("invalid session")
}

// Load everything needed to start the web interface
func (s *Backend) RunHTTPServer(ip string, port int, creds []*pb.OAuth) {
	Website.Creds = creds

	listen := fmt.Sprintf("%s:%d", ip, port)
	r := LoadWebsiteRoutes()

	httpsrv := &http.Server{
		Handler:      r,
		Addr:         listen,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	s.Logf(LogLevelNormal, "Listening for web requests on http://%s\n", listen)
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

	page, err := dashboardPage(u)
	if err != nil {
		log.Println(err)
		fmt.Fprintln(w, "error 500")
		return
	}

	tmpl, e := template.New("dashboard").Funcs(funcMap).ParseFiles(
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-header.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "dashboard.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-footer.tmpl"),
	)

	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "dashboard", page)
		if err != nil {
			log.Println(err)
		}
	}
}

func dashboardPage(user *pb.User) (PageResponse, error) {
	var out PageResponse
	if user == nil {
		return out, fmt.Errorf("null user building dashboard page")
	}
	out.SessionUser = user
	out.Head.Title = "Dashboard | CloudAdmin"
	out.Frontends = FrontendsByIdentity(user.GetEmail())
	return out, nil
}

// Displays info page for a particular client
func WebsiteHandlerServerView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["ServerUUID"]
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	fe, err := be.FindFrontend(uuid)
	if err != nil {
		log.Println("invalid server id:", uuid)
		return
	}

	/*
		data := WebpageData{
			Title:       fe.Name + " management | Q2Admin CloudAdmin",
			HeaderTitle: fe.Name,
			SessionUser: user,
			Frontend:    fe,
		}
		data.NavHighlight.Servers = "active"
	*/

	data := PageResponse{}
	data.Head.Title = fmt.Sprintf("%s | Q2Admin CloudAdmin", fe.Name)
	data.Head.Keywords = ""
	data.Title = fe.Name
	data.SessionUser = user
	data.Frontend = fe

	data.NavHighlight.Servers = "active"
	tmpl, e := template.New("dashboard").Funcs(funcMap).ParseFiles(
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-header.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "server-view.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-footer.tmpl"),
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
	infile := path.Join(be.config.GetWebRoot(), "templates", "new", "sign-in.tmpl")
	tmpl, e := template.ParseFiles(infile)

	var page PageResponse
	for i := range Website.Creds {
		page.AuthProviders = append(page.AuthProviders, AuthProvider{
			URL:     BuildAuthURL(Website.Creds[i], i),
			Icon:    Website.Creds[i].GetImagePath(),
			Alt:     Website.Creds[i].GetAlternateText(),
			Enabled: !Website.Creds[i].GetDisabled(),
		})
	}
	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, page)
	}
}

func WebsiteAPIGetConnectedServers(w http.ResponseWriter, r *http.Request) {
	var activeservers []ActiveServer
	for _, s := range be.frontends {
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
	cookie := http.Cookie{Name: WebCookieName, Value: "", Expires: expire}
	http.SetCookie(w, &cookie)
}

// Log a user out
func WebSignout(w http.ResponseWriter, r *http.Request) {
	AuthLogout(w, r)
	http.Redirect(w, r, Routes.Index, http.StatusFound)
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
		path.Join(be.config.GetWebRoot(), "templates", "header-main.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "my-groups.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "footer.tmpl"),
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

	fes := FrontendsByIdentity(user.Email)

	data := PageResponse{}
	data.Head.Title = "My Servers | Q2Admin CloudAdmin"
	data.Head.Keywords = ""
	data.Title = "My Servers"
	data.SessionUser = user
	data.Frontends = fes
	data.FrontendCount = len(fes)

	data.NavHighlight.Servers = "active"

	tmpl, e := template.ParseFiles(
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-header.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "servers.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-footer.tmpl"),
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
		path.Join(be.config.GetWebRoot(), "templates", "header-main.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "privacy-policy.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "footer.tmpl"),
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
	data := PageResponse{}
	data.Head.Title = "Terms of Use | Q2Admin CloudAdmin"
	data.Head.Keywords = "terms"
	data.Title = "Terms of Use"
	data.SessionUser = user

	tmpl, e := template.ParseFiles(
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-header.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "terms-of-use.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-footer.tmpl"),
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

func boolToYesNo(val bool) string {
	if val {
		return fmt.Sprintf("<span class=%q>yes</span>", "text-success")
	}
	return fmt.Sprintf("<span class=%q>no</span>", "text-danger")
}

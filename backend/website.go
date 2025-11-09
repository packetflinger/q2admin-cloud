package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/packetflinger/libq2/flags"
	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/frontend"
	"github.com/packetflinger/q2admind/util"

	pb "github.com/packetflinger/q2admind/proto"
)

const (
	CookieName = "q2asess"
)

var (
	Website = WebInterface{}
	funcMap = template.FuncMap{
		"yesno":      boolToYesNo,
		"yesnoemoji": boolToEmoji,
		"checked":    boolToChecked,
		"ago":        util.TimeAgo,
		"dmflags":    dmflags,
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
	Creds  []*pb.OAuth
	Auths  []AuthProvider
	Secret []byte // for signing/verifying JWTs
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
	cookie, e := r.Cookie(CookieName)
	if e != nil {
		return nil, e
	}
	user, e := ValidateSessionToken(cookie.Value, Website.Secret)
	if e != nil {
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

// Create a JSON web token to write as the cookie data for the session. The `u`
// parameter is the user this session is for, `id` is just a unique identifier
// for the session, `length` is the number of seconds from now the session
// should be valid for, and `secret` is a key used to cryptographically sign
// the token to ensure the integrity of the claims.
func CreateSessionToken(u *pb.User, id string, length int64, secret []byte) (string, error) {
	if u == nil {
		return "", fmt.Errorf("can't create session token: nil user")
	}
	if len(secret) == 0 {
		return "", fmt.Errorf("can't create session token: empty secret")
	}
	now := time.Now().Unix()
	claims := jwt.StandardClaims{
		ExpiresAt: now + length,
		Id:        id,
		IssuedAt:  now,
		Subject:   u.Email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}
	return ss, nil
}

// Ensure the JWT is valid
// 1. Crypto signature matches
// 2. Token is not yet expired
// 3. Token Id is not in our internal revocation list (todo)
func ValidateSessionToken(token string, secret []byte) (*pb.User, error) {
	claims := &jwt.StandardClaims{}
	t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to parse session token")
	}
	if t.Valid {
		for i := range be.users {
			u := be.users[i]
			if u.Email == claims.Subject {
				return u, nil
			}
		}
		return nil, fmt.Errorf("unknown session user: %s", claims.Subject)
	}
	return nil, fmt.Errorf("invalid session token")
}

// Make sure the session presented is valid.
// 1. Current date is after the session creation date
// 2. Current date is before the session expiration
func ValidateSession(sess string) (*pb.User, error) {
	_, err := ValidateSessionToken(sess, Website.Secret)
	if err != nil {
		return nil, err
	}
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
func (s *Backend) RunHTTPServer(ip string, port int, creds []*pb.OAuth, secret []byte) {
	Website.Creds = creds
	Website.Secret = secret

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
	cookie := http.Cookie{Name: CookieName, Value: "", Expires: expire}
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

func ChangeUUIDHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	currentUUID := mux.Vars(r)["ServerUUID"]
	f, err := be.FindFrontend(currentUUID)
	if err != nil {
		fmt.Fprintln(w, "error 500 - error finding frontend")
		return
	}
	if f.Owner != user.Email {
		fmt.Fprintln(w, "403 - unauthorized")
		return
	}
	newUUID := uuid.NewString()
	qry := `UPDATE frontend SET uuid = ? WHERE uuid = ?`
	res, err := db.Handle.Exec(qry, newUUID, currentUUID)
	if err != nil {
		ar, err := res.RowsAffected()
		if err != nil {
			fmt.Fprintln(w, "500 - db update failed")
			log.Println("Error updating UUID:", err)
			return
		}
		if ar != 1 {
			fmt.Fprintln(w, "500 - db update failed")
			log.Println("Error updating UUID: affected row != 1")
			return
		}
	}
	f.UUID = newUUID
	if err = f.Materialize(); err != nil {
		fmt.Fprintln(w, "error 500 - error saving frontend")
		return
	}
	http.Redirect(w, r, path.Join("/sv", f.UUID, f.Name), http.StatusSeeOther)
}

func WebEditServer(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	uuid := r.PostFormValue("uuid")
	if uuid == "" {
		fmt.Fprintln(w, "error 500 - bad form submission")
		return
	}
	name := r.PostFormValue("srvname")
	if name == "" {
		fmt.Fprintln(w, "error 500 - bad form submission")
		return
	}
	addr := r.PostFormValue("serveraddr")
	if addr == "" {
		fmt.Fprintln(w, "500 - empty frontend address")
		return
	}
	enabled := r.PostFormValue("switchenabled") == "on"
	teleport := r.PostFormValue("switchteleport") == "on"
	invite := r.PostFormValue("switchinvite") == "on"
	f, err := be.FindFrontend(uuid)
	if err != nil {
		log.Println(err)
		fmt.Fprintln(w, "error 500 - bad frontend lookup")
		return
	}

	if f.Owner != user.Email {
		fmt.Fprintln(w, "403 - permission denied")
		return
	}
	tokens := strings.Split(addr, ":")
	if len(tokens) == 2 {
		f.IPAddress = tokens[0]
		port, err := strconv.Atoi(tokens[1])
		if err != nil {
			port = 27910
		}
		f.Port = port
	}
	f.Enabled = enabled
	f.AllowTeleport = teleport
	f.AllowInvite = invite
	if err = f.Materialize(); err != nil {
		fmt.Fprintln(w, "error 500 - error saving frontend")
		return
	}
	http.Redirect(w, r, path.Join("/sv", uuid, name), http.StatusSeeOther)
}

func PlayerViewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{}
	data.Head.Title = "Player View | Q2Admin CloudAdmin"
	data.Head.Keywords = "Player"
	data.Title = "Player View"
	data.SessionUser = user

	tmpl, e := template.ParseFiles(
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-header.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "player-view.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-footer.tmpl"),
	)
	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "player-view", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func ServerConsoleHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{}
	data.Head.Title = "Server Console | Q2Admin CloudAdmin"
	data.Head.Keywords = "console"
	data.Title = "Server Console"
	data.SessionUser = user

	tmpl, e := template.ParseFiles(
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-header.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "terminal.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-footer.tmpl"),
	)
	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "terminal", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{}
	data.Head.Title = "Search | Q2Admin CloudAdmin"
	data.Title = "Search"
	data.SessionUser = user

	tmpl, e := template.ParseFiles(
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-header.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "search.tmpl"),
		path.Join(be.config.GetWebRoot(), "templates", "new", "common-footer.tmpl"),
	)
	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "search", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func RedirectToSignon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, Routes.AuthLogin, http.StatusSeeOther) // 303
}

// Given a boolean, return html for a green "yes" or red "no"
func boolToYesNo(val bool) template.HTML {
	if val {
		return template.HTML(fmt.Sprintf("<span class=%q>yes</span>", "text-success"))
	}
	return template.HTML(fmt.Sprintf("<span class=%q>no</span>", "text-danger"))
}

// Given a boolean, return a green checkbox emoji or red X
func boolToEmoji(val bool) template.HTML {
	if val {
		return template.HTML("&#x2705;") // green checkmark
	}
	// &#x2715; also good
	return template.HTML("&#10006;")
}

// for translating boolean value from struct to an HTML checkbox value
func boolToChecked(val bool) string {
	if val {
		return "checked"
	}
	return ""
}

// convert a string representation of dmflags bitmask and show the values.
func dmflags(val string) string {
	fl, err := strconv.Atoi(val)
	if err != nil {
		return "invalid"
	}
	return flags.ToString(fl)
}

// Implement the managment website for Cloudadmin.
//
// Whether this HTTP service is started depends on the `api_enable` property
// in the main config proto. This service does not use TLS, is just straight
// up HTTP. It should use an ephemeral port (not 80 or 443) and be placed
// behind an appropriate application-layer firewall/proxy like NGINX that
// implements HTTPS for production use.
package backend

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
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
	AuthBypass = "joe@joereid.com" // for testing, skips auth
)

var (
	Website = WebInterface{}
	funcMap = template.FuncMap{
		"yesno":      boolToYesNo,
		"yesnoemoji": boolToEmoji,
		"checked":    boolToChecked,
		"ago":        util.TimeAgo,
		"dmflags":    dmflags,
		"datetime":   util.TimeDateString,
		"online":     playersOnline,
		"summarize":  summarize,
		"join":       strings.Join,
	}
)

type PlayerDatabaseInfo struct {
	ConnectTime int64
	Cookie      string
	Hostname    string
	Id          int
	IP          string
	Name        string
	ServerUUID  string
	Server_id   int
	Userinfo    string
	Version     string
	Vpn         bool
}

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
	SearchResults []frontend.SearchResult
	SearchQuery   string
	PlayerDBInfo  PlayerDatabaseInfo
	Form          url.Values // submitted form values, to refill a form after an error
	FormError     string     // why the submitted form was rejected
	Notice        *Notice    // result of the action that redirected here
	Rule          *pb.Rule   // the rule to view/edit
	Rules         []*pb.Rule // a list of rules (srv level)
	Player        *frontend.Player
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
	if r == nil {
		return nil, fmt.Errorf("nil request")
	}
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

// Create a JSON web token to write as the cookie data for the session.
//
// Parameters:
//
//	`u`      is the user this session is for
//	`id`     is just a unique identifier for the session
//	`length` is the number of seconds the session should be valid for
//	`secret` is a key used to cryptographically sign the token
//
// Returns a token string that is marshalled JSON with a signature appended to
// it and base64 encoded. This should be used for the contents of the session
// cookie.
func CreateSessionToken(u *pb.User, id string, length int64, secret []byte) (string, error) {
	if u == nil {
		return "", fmt.Errorf("can't create session token: nil user")
	}
	if len(secret) == 0 {
		return "", fmt.Errorf("can't create session token: empty secret")
	}
	if id == "" {
		id = uuid.NewString() // bad practice to change func params
	}
	if length < 600 {
		length = 600 // anything less than 10 minutes is kind of stupid
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
//
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

var serverNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// validateNewServer checks the add-server form. The name ends up in URLs and
// in a directory path, so it's restricted to lowercase letters, digits and
// dashes (spaces become dashes). An empty port means the Quake 2 default.
func validateNewServer(name, address, port string) (string, int, error) {
	name = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "-")
	if !serverNameRE.MatchString(name) {
		return "", 0, errors.New("name must be 1-32 letters, numbers or dashes")
	}
	address = strings.TrimSpace(address)
	if net.ParseIP(address) == nil && (address == "" || strings.ContainsAny(address, " \t/:") || len(address) > 253) {
		return "", 0, errors.New("address must be an IP address or hostname, without a port")
	}
	if port == "" {
		return name, 27910, nil
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return "", 0, errors.New("port must be a number from 1 to 65535")
	}
	return name, p, nil
}

// playersOnline counts fe's occupied player slots. PlayerCount isn't used
// because it isn't reset when a frontend reconnects.
func playersOnline(fe *frontend.Frontend) int {
	n := 0
	for _, p := range fe.Players {
		if p.ConnectTime > 0 {
			n++
		}
	}
	return n
}

// FrontendSummary is the dashboard's totals across a user's frontends.
type FrontendSummary struct {
	Servers, Online, Players, Rules int
}

// summarize totals fes for the dashboard. Call it from a template rendered by
// executeLocked so the frontends are locked.
func summarize(fes []*frontend.Frontend) FrontendSummary {
	s := FrontendSummary{Servers: len(fes)}
	for _, fe := range fes {
		if fe.Connected && fe.Trusted {
			s.Online++
			s.Players += playersOnline(fe)
		}
		s.Rules += len(fe.Rules)
	}
	return s
}

// parsePage loads a page template along with the shared header, footer and
// template functions.
func parsePage(file string) (*template.Template, error) {
	dir := path.Join(be.config.GetWebRoot(), "templates", "new")
	return template.New(file).Funcs(funcMap).ParseFiles(
		path.Join(dir, "common-header.tmpl"),
		path.Join(dir, file),
		path.Join(dir, "common-footer.tmpl"),
	)
}

// executeLocked renders a page while holding the given frontends' locks, then
// writes it after unlocking so a slow client can't stall a game server. Pass
// frontends in Frontends() order: their relative order never changes, which
// keeps holding several at once deadlock-free.
func executeLocked(w io.Writer, tmpl *template.Template, name string, data any, fes ...*frontend.Frontend) error {
	var held []*frontend.Frontend
	for _, f := range fes {
		if f != nil && !slices.Contains(held, f) {
			f.Mu.Lock()
			held = append(held, f)
		}
	}
	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, name, data)
	for _, f := range held {
		f.Mu.Unlock()
	}
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w)
	return err
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

func dashboardPage(user *pb.User) (PageResponse, error) {
	var out PageResponse
	if user == nil {
		return out, fmt.Errorf("null user building dashboard page")
	}
	out.SessionUser = user
	out.Head.Title = "Dashboard | CloudAdmin"
	out.Frontends = FrontendsByIdentity(user.GetEmail())
	out.NavHighlight.Dashboard = "active"
	return out, nil
}

// Displays info page for a particular client
func WebsiteHandlerServerView(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe, err := be.FindFrontend(mux.Vars(r)["ServerUUID"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	renderServerView(w, user, fe, notices[r.URL.Query().Get("notice")], "")
}

// renderServerView shows fe's page, optionally with a notice from a previous
// action or the reason a settings change was rejected.
func renderServerView(w http.ResponseWriter, user *pb.User, fe *frontend.Frontend, notice *Notice, formError string) {
	data := PageResponse{SessionUser: user, Frontend: fe, Notice: notice, FormError: formError}
	fe.Mu.Lock()
	data.Head.Title = fe.Name + " | CloudAdmin"
	fe.Mu.Unlock()
	data.NavHighlight.Servers = "active"
	tmpl, err := parsePage("server-view.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "server-view", data, fe); err != nil {
		log.Println(err)
	}
}

// Notice is a one-line status message shown at the top of a page after an
// action redirects back to it.
type Notice struct {
	Class string // bootstrap alert color: success, warning, danger
	Text  string
}

// notices are looked up by the "notice" query parameter, so pages only ever
// show these fixed messages.
var notices = map[string]*Notice{
	"map":     {"success", "Map change sent. The current map updates once the server reports it."},
	"badmap":  {"danger", "Map names can only contain letters, numbers, dashes, underscores and slashes."},
	"offline": {"warning", "That needs the server to be connected."},
	"saved":   {"success", "Settings saved."},
	"uuid":    {"success", "New UUID generated. Update the server's q2admin config to use it."},
	"key":     {"success", "Public key saved."},
	"badkey":  {"danger", "That isn't a valid PEM-encoded RSA public key."},
}

var mapNameRE = regexp.MustCompile(`^[A-Za-z0-9_/-]{1,64}$`)

// ChangeMapHandler tells a connected frontend to switch maps.
func ChangeMapHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := ownedFrontend(w, r, user)
	if fe == nil {
		return
	}
	mapname := strings.TrimSpace(r.PostFormValue("map"))
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	notice := "map"
	switch {
	case !mapNameRE.MatchString(mapname): // it's sent as a console command
		notice = "badmap"
	case !(fe.Connected && fe.Trusted):
		notice = "offline"
	default:
		ConsoleCommand(fe, "gamemap "+mapname)
		SendMessages(fe)
	}
	http.Redirect(w, r, path.Join("/sv", fe.UUID, fe.Name)+"?notice="+notice, http.StatusSeeOther)
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
	if AuthBypass != "" {
		MockLogin(w, r, AuthBypass)
		return
	}
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

func WebAddServer(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	data := PageResponse{SessionUser: user}
	data.Head.Title = "Add Server | CloudAdmin"
	data.NavHighlight.Servers = "active"

	// The add button was pressed to submit the form
	if r.PostFormValue("AddButton") != "" {
		name, port, err := validateNewServer(r.PostFormValue("NewServerName"), r.PostFormValue("NewServerAddress"), r.PostFormValue("NewServerPort"))
		if err == nil {
			if _, dupe := be.FindFrontendByName(name); dupe == nil {
				err = fmt.Errorf("a server named %q already exists", name)
			}
		}
		if err == nil {
			f := &frontend.Frontend{
				IPAddress: strings.TrimSpace(r.PostFormValue("NewServerAddress")),
				Name:      name,
				Owner:     user.Email,
				Path:      path.Join(be.config.ClientDirectory, name),
				Port:      port,
				UUID:      uuid.NewString(),
			}
			if err = be.CreateFrontend(f); err == nil {
				http.Redirect(w, r, path.Join("/sv", f.UUID, f.Name), http.StatusSeeOther)
				return
			}
			log.Printf("error creating frontend %q: %v\n", name, err)
			err = errors.New("unable to save the server, try again")
		}
		data.FormError = err.Error()
		data.Form = r.PostForm
	}

	tmpl, e := parsePage("server-add.tmpl")

	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "server-add", data)
		if err != nil {
			log.Println(err)
		}
	}
}

// Handler to delete a user's frontend
func WebDelServer(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe, err := be.FindFrontend(mux.Vars(r)["ServerUUID"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fe.Mu.Lock()
	owner, name, protected := fe.Owner, fe.Name, fe.DeleteProtect
	fe.Mu.Unlock()
	if owner != user.Email {
		http.Error(w, "403 - not owner", http.StatusForbidden)
		be.Logf(LogLevelInfo, "Non-owner %q attempt to delete %q", user.Email, name)
		return
	}
	if protected {
		http.Error(w, "403 - delete protection is on, turn it off in settings first", http.StatusForbidden)
		return
	}
	err = be.DeleteFrontend(name)
	if err != nil {
		http.Error(w, "500 - error deleting frontend", http.StatusInternalServerError)
		be.Logf(LogLevelInfo, "%s", err)
		return
	}
	http.Redirect(w, r, "/my-servers", http.StatusSeeOther)
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

	tmpl, e := parsePage("terms-of-use.tmpl")
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
		http.NotFound(w, r)
		return
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Owner != user.Email {
		http.Error(w, "403 - unauthorized", http.StatusForbidden)
		return
	}
	newUUID := uuid.NewString()
	res, err := db.Handle.Exec(`UPDATE frontend SET uuid = ? WHERE uuid = ?`, newUUID, currentUUID)
	if err == nil {
		var n int64
		if n, err = res.RowsAffected(); err == nil && n != 1 {
			err = fmt.Errorf("%d rows affected", n)
		}
	}
	if err != nil {
		http.Error(w, "500 - db update failed", http.StatusInternalServerError)
		log.Println("Error updating UUID:", err)
		return
	}
	be.frontendsMu.Lock() // lookups match on UUID
	f.UUID = newUUID
	be.frontendsMu.Unlock()
	http.Redirect(w, r, path.Join("/sv", f.UUID, f.Name)+"?notice=uuid#settings", http.StatusSeeOther)
}

func WebEditServer(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	f, err := be.FindFrontend(r.PostFormValue("uuid"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name, port, err := validateNewServer(r.PostFormValue("servername"), r.PostFormValue("serveraddr"), r.PostFormValue("serverport"))
	if err == nil {
		if other, dupe := be.FindFrontendByName(name); dupe == nil && other != f {
			err = fmt.Errorf("a server named %q already exists", name)
		}
	}
	if err != nil {
		renderServerView(w, user, f, nil, err.Error())
		return
	}
	addr := strings.TrimSpace(r.PostFormValue("serveraddr"))
	enabled := r.PostFormValue("switchenabled") == "on"
	teleport := r.PostFormValue("switchteleport") == "on"
	invite := r.PostFormValue("switchinvite") == "on"
	protect := r.PostFormValue("switchprotect") == "on"

	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Owner != user.Email {
		http.Error(w, "403 - permission denied", http.StatusForbidden)
		return
	}
	// save first so memory doesn't diverge from the database on failure
	qry := "UPDATE frontend SET name=?, ip_address=?, port=?, enabled=?, allow_teleport=?, allow_invite=?, delete_protection=? WHERE id=?"
	if _, err = db.Handle.Exec(qry, name, addr, port, enabled, teleport, invite, protect, f.ID); err != nil {
		http.Error(w, "500 - error updating frontend in database", http.StatusInternalServerError)
		log.Printf("error updating frontend %q: %v\n", f.Name, err)
		return
	}
	f.IPAddress = addr
	f.Port = port
	f.Enabled = enabled
	f.AllowTeleport = teleport
	f.AllowInvite = invite
	f.DeleteProtect = protect
	be.frontendsMu.Lock() // lookups match on Name
	f.Name = name
	be.frontendsMu.Unlock()
	http.Redirect(w, r, path.Join("/sv", f.UUID, name)+"?notice=saved#settings", http.StatusSeeOther)
}

func PlayerViewHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["ServerUUID"]
	playerId := vars["ClientNum"]
	fe, err := be.FindFrontend(uuid)
	if err != nil {
		log.Println("invalid server id:", uuid)
		return
	}
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	pid, err := strconv.Atoi(playerId)
	if err != nil {
		be.Logf(LogLevelInfo, "invalid player id %q: %v", playerId, err)
		fmt.Fprintf(w, "invalid player id %q", playerId)
		return
	}
	fe.Mu.Lock()
	if pid < 0 || pid >= len(fe.Players) {
		fe.Mu.Unlock()
		be.Logf(LogLevelInfo, "invalid player id %q - not a valid client slot", playerId)
		fmt.Fprintf(w, "invalid player id %q", playerId)
		return
	}
	data := PageResponse{}
	data.Player = &(fe.Players[pid])
	fe.Mu.Unlock()
	data.Head.Title = "Player View | Q2Admin CloudAdmin"
	data.Head.Keywords = "Player"
	data.Title = "Player View"
	data.SessionUser = user
	data.Frontend = fe

	tmpl, e := parsePage("player-view.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		err = executeLocked(w, tmpl, "player-view", data, fe)
		if err != nil {
			log.Println(err)
		}
	}
}

// ServerConsoleHandler shows the live console page for a frontend.
func ServerConsoleHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe, err := be.FindFrontend(mux.Vars(r)["ServerUUID"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data := PageResponse{SessionUser: user, Frontend: fe}
	fe.Mu.Lock()
	data.Head.Title = "Console: " + fe.Name + " | CloudAdmin"
	fe.Mu.Unlock()
	data.NavHighlight.Servers = "active"
	tmpl, err := parsePage("terminal.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "terminal", data, fe); err != nil {
		log.Println(err)
	}
}

// ownedFrontend finds the frontend named in the URL and checks user owns it,
// writing the error response and returning nil if not.
func ownedFrontend(w http.ResponseWriter, r *http.Request, user *pb.User) *frontend.Frontend {
	fe, err := be.FindFrontend(mux.Vars(r)["ServerUUID"])
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	fe.Mu.Lock()
	owner := fe.Owner
	fe.Mu.Unlock()
	if owner != user.Email {
		http.Error(w, "403 - permission denied", http.StatusForbidden)
		return nil
	}
	return fe
}

// ConsoleStreamHandler streams a frontend's console output to the browser as
// server-sent events: the same feed SSH sessions watch.
func ConsoleStreamHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		http.Error(w, "401 - signed out", http.StatusUnauthorized)
		return
	}
	fe := ownedFrontend(w, r, user)
	if fe == nil {
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the server's WriteTimeout would cut the stream off
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	stream := make(chan string, 64) // SSHPrintln drops messages when this is full
	fe.AddTerminal(&stream)
	defer fe.TerminalDisconnected(&stream)
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	fmt.Fprint(w, ": connected\n\n")
	for {
		if err := rc.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
		case msg := <-stream:
			data, _ := json.Marshal(msg) // one line, whatever msg contains
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
	}
}

// ConsoleCommandHandler runs one console command on a frontend, as if typed
// at its server console. Output arrives through ConsoleStreamHandler.
func ConsoleCommandHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		http.Error(w, "401 - signed out", http.StatusUnauthorized)
		return
	}
	fe := ownedFrontend(w, r, user)
	if fe == nil {
		return
	}
	cmd := strings.TrimSpace(r.PostFormValue("cmd"))
	// a newline would smuggle in extra commands
	if cmd == "" || len(cmd) > 256 || strings.ContainsAny(cmd, "\r\n") {
		http.Error(w, "send one command of up to 256 characters", http.StatusBadRequest)
		return
	}
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	if !(fe.Connected && fe.Trusted) {
		http.Error(w, "the server is offline", http.StatusConflict)
		return
	}
	ConsoleCommand(fe, cmd)
	SendMessages(fe)
	fe.Log.Printf("web console [%s]: %s\n", user.GetEmail(), cmd)
	fe.SSHPrintln(fmt.Sprintf("[%s] > %s", user.GetEmail(), cmd)) // other watchers see it too
	w.WriteHeader(http.StatusNoContent)
}

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	/*
		user, err := GetSessionUser(r)
		if err != nil {
			RedirectToSignon(w, r)
			return
		}
		q := r.URL.Query().Get("q")
		if q == "" {
			fmt.Fprintf(w, "500 - blank search query")
			return
		}
		_, err = be.Search(r.Context(), q)
		if err != nil {
			fmt.Fprintf(w, "500 - search failed")
			fmt.Printf("error searching %q on %q: %v\n", q, f.Name, err)
			return
		}
		data := PageResponse{}
		data.Head.Title = "Search | Q2Admin CloudAdmin"
		data.Title = "Search"
		data.SessionUser = user
		// data.SearchResults = results
		// data.Frontend = f
		data.SearchQuery = q

		tmpl, e := template.New("searchresults").Funcs(funcMap).ParseFiles(
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
	*/
}

func ServerSearchHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	vars := mux.Vars(r)
	uuid := vars["UUID"]
	f, err := be.FindFrontend(uuid)
	if err != nil {
		fmt.Fprintf(w, "500 - error looking up frontend: %q", uuid)
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		fmt.Fprintf(w, "500 - blank search query")
		return
	}
	f.Mu.Lock() // Search reads f.UUID
	results, err := f.Search(r.Context(), q)
	f.Mu.Unlock()
	if err != nil {
		fmt.Fprintf(w, "500 - search failed")
		fmt.Printf("error searching %q on %q: %v\n", q, f.Name, err)
		return
	}
	data := PageResponse{}
	data.Head.Title = "Search | Q2Admin CloudAdmin"
	data.Title = "Search"
	data.SessionUser = user
	data.SearchResults = results
	data.Frontend = f
	data.SearchQuery = q

	tmpl, e := parsePage("search.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		err = executeLocked(w, tmpl, "search", data, f)
		if err != nil {
			log.Println(err)
		}
	}
}

func PlayerSearchViewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	vars := mux.Vars(r)
	lookup := vars["lookup"]
	lu, err := base64.StdEncoding.DecodeString(lookup)
	if err != nil {
		fmt.Fprintf(w, "500 - can't decode lookup string")
		fmt.Printf("Error decoding %q for player lookup\n", lookup)
		return
	}
	tokens := strings.Split(string(lu), "\\")
	if len(tokens) != 2 {
		fmt.Fprintf(w, "500 - invalid lookup string")
		fmt.Printf("Error: player lookup %q not properly formatted\n", string(lu))
		return
	}
	nameLookup := tokens[0]
	timeLookup := tokens[1]
	qry := `
	SELECT p.*, f.uuid
	FROM player AS p
	JOIN frontend AS f ON f.id = p.server_id
	WHERE p.name = ? AND p.time = ?
	LIMIT 1`
	res := db.Handle.QueryRow(qry, nameLookup, timeLookup)
	var p PlayerDatabaseInfo
	err = res.Scan(&p.Id, &p.Server_id, &p.Name, &p.IP, &p.Hostname, &p.Vpn, &p.Cookie, &p.Version, &p.Userinfo, &p.ConnectTime, &p.ServerUUID)
	if err != nil {
		fmt.Fprintf(w, "500 - error scanning player data")
		fmt.Printf("Error: scanning player data: %v\n", err)
		return
	}
	data := PageResponse{}
	data.Head.Title = "Player | Q2Admin CloudAdmin"
	data.Title = "Player: " + nameLookup
	data.SessionUser = user
	data.PlayerDBInfo = p
	tmpl, e := parsePage("player.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "player", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func RuleViewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{}
	data.Head.Title = "Rule View | Q2Admin CloudAdmin"
	data.Title = "Rule"
	data.SessionUser = user

	lookup := mux.Vars(r)["uuid"]
	var owner *frontend.Frontend // whose Rules the page renders
	for _, f := range be.UserFrontends(user.GetEmail()) {
		f.Mu.Lock()
		for _, r := range f.Rules {
			if r.GetUuid() == lookup {
				data.Rules = f.Rules
				data.Rule = r
				owner = f
			}
		}
		f.Mu.Unlock()
	}

	tmpl, e := parsePage("rule-view.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		err = executeLocked(w, tmpl, "rule", data, owner)
		if err != nil {
			log.Println(err)
		}
	}
}

func RuleEditHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{}
	//data.Head.Title = "Frontend Rules | Q2Admin CloudAdmin"
	//data.Title = "Rules"
	data.SessionUser = user

	lookup := mux.Vars(r)["uuid"]
	//fes := be.UserFrontends(user.GetEmail())
	for _, f := range be.Frontends() {
		f.Mu.Lock()
		for j, r := range f.Rules {
			if r.GetUuid() == lookup {
				data.Frontend = f
				data.Rule = f.Rules[j]
			}
		}
		f.Mu.Unlock()
	}

	tmpl, e := parsePage("rule-edit.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		err = executeLocked(w, tmpl, "rule-edit", data, data.Frontend)
		if err != nil {
			log.Println(err)
		}
	}
}

func RuleAddHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{}
	data.Head.Title = "Frontend Rules | Q2Admin CloudAdmin"
	data.Title = "Rules"
	data.SessionUser = user

	tmpl, e := parsePage("rule-add.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "rule-add", data)
		if err != nil {
			log.Println(err)
		}
	}
}

func RuleListHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{}
	data.Head.Title = "Frontend Rules | Q2Admin CloudAdmin"
	data.Title = "Rules"
	data.SessionUser = user

	lookup := mux.Vars(r)["ServerUUID"]
	fes := be.UserFrontends(user.GetEmail())
	for _, f := range fes {
		f.Mu.Lock()
		if f.UUID == lookup {
			data.Frontend = f
			data.Frontend.Rules = SortRules(data.Frontend.Rules)
		}
		f.Mu.Unlock()
		if data.Frontend != nil {
			break
		}
	}

	tmpl, e := parsePage("rule-list.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		err = executeLocked(w, tmpl, "rule-list", data, data.Frontend)
		if err != nil {
			log.Println(err)
		}
	}
}

func ServerKeysHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	// handle saving public key
	if r.PostFormValue("action") == "SaveKey" {
		uuid := r.PostFormValue("uuid")
		name := r.PostFormValue("srvname")
		keydata := r.PostFormValue("keydata")
		if uuid == "" || name == "" || keydata == "" {
			fmt.Fprintln(w, "error 500 - bad form submission")
			return
		}
		f, err := be.FindFrontend(uuid)
		if err != nil {
			log.Println(err)
			fmt.Fprintln(w, "error 500 - bad frontend lookup")
			return
		}
		f.Mu.Lock()
		if f.Owner != user.Email {
			f.Mu.Unlock()
			fmt.Fprintln(w, "403 - permission denied")
			return
		}
		f.PublicKeyData = strings.Trim(keydata, " \r\n\t")
		keyData := f.PublicKeyData
		f.Mu.Unlock()
		dest := path.Join(be.config.ClientDirectory, name, "key")
		err = os.WriteFile(dest, []byte(keyData), 0600)
		if err != nil {
			fmt.Fprintf(w, "error 500 - unable to write key file")
			return
		}
		http.Redirect(w, r, path.Join("/sv", uuid, name), http.StatusSeeOther)
		return
	}

	data := PageResponse{}
	data.Head.Title = "Generate Encyption Key Pair | Q2Admin CloudAdmin"
	data.Title = "Keys"
	data.SessionUser = user

	lookup := mux.Vars(r)["uuid"]
	var owner *frontend.Frontend // whose Rules the page renders
	for _, f := range be.UserFrontends(user.GetEmail()) {
		f.Mu.Lock()
		for _, r := range f.Rules {
			if r.GetUuid() == lookup {
				data.Rules = f.Rules
				data.Rule = r
				owner = f
			}
		}
		f.Mu.Unlock()
	}

	tmpl, e := parsePage("generate-keys.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		err = executeLocked(w, tmpl, "keygen", data, owner)
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
	return template.HTML("&#x274C;") // red x
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

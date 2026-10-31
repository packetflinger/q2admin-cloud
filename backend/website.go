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
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/packetflinger/q2admind/api/website"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/packetflinger/libq2/flags"
	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/frontend"
	"github.com/packetflinger/q2admind/util"

	pb "github.com/packetflinger/q2admind/proto"
	"google.golang.org/protobuf/proto"
)

const (
	CookieName = "q2asess"
	// How long a website sign-in lasts, independent of the OAuth provider's
	// access token (Google's expire after an hour).
	SessionLength = 24 * time.Hour
	AuthBypass = "" // for testing, skips auth
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
		"userinfo":   frontend.UserinfoMap,
		"lines":      func(l []string) string { return strings.Join(l, "\n") },
		"uilines":    userinfoLines,
		"spans":      func() any { return StatSpans },
		"duration":   playDuration,
		"pct":        slicePercent,
		"pieArgs": func(title, id, unit string, slices []Slice) any {
			return struct {
				Title, ID, Unit string
				Slices          []Slice
			}{title, id, unit, slices}
		},
		"displayName": DisplayName,
		"inGroup":     func(g *ServerGroup, fe *frontend.Frontend) bool { return slices.Contains(g.Members, fe) },
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
	Form          url.Values        // submitted form values, to refill a form after an error
	FormError     string            // why the submitted form was rejected
	Notice        *Notice           // result of the action that redirected here
	ServerNames   map[string]string // UUID -> name of the user's servers
	KeyPair       *KeyPair          // just generated, for the keys page
	Groups        []*ServerGroup
	Group         *ServerGroup
	Stats         *Stats
	RuleHome      *RuleHome  // what the rule being shown belongs to
	GroupRules    []*pb.Rule // rules from groups that also apply to Frontend
	CanEdit       bool       // the user owns Frontend, so can change its settings
	Rule          *pb.Rule   // the rule to view/edit
	Rules         []*pb.Rule // a list of rules (srv level)
	Player        *frontend.Player
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
	Name    string // short provider name for the button, like "Google"
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

// userinfoLines formats a rule's userinfo patterns for the rule form.
func userinfoLines(uis []*pb.UserInfo) string {
	var l []string
	for _, ui := range uis {
		l = append(l, ui.GetProperty()+"="+ui.GetValue())
	}
	return strings.Join(l, "\n")
}

// slicePercent is v's share of all the slices, like "42%".
func slicePercent(v int, slices []Slice) string {
	total := 0
	for _, s := range slices {
		total += s.Value
	}
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d%%", (v*100+total/2)/total)
}

// playDuration formats seconds of play like "2h 05m" or "12m".
func playDuration(secs int64) string {
	if secs < 60 {
		return "<1m"
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm", secs/60)
	}
	return fmt.Sprintf("%dh %02dm", secs/3600, secs%3600/60)
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

// parsePage loads a page template along with the shared header, footer,
// stats widgets, rules table and template functions, from the files built
// into the binary.
func parsePage(file string) (*template.Template, error) {
	return template.New(file).Funcs(funcMap).ParseFS(website.Files,
		"templates/new/common-header.tmpl",
		"templates/new/common-stats.tmpl",
		"templates/new/common-rules.tmpl",
		"templates/new/"+file,
		"templates/new/common-footer.tmpl",
	)
}

// providerName turns an OAuth type like "GOOGLE" into "Google".
func providerName(t string) string {
	if t == "" {
		return ""
	}
	return t[:1] + strings.ToLower(t[1:])
}

// DisplayName is how a user is shown to others, such as in console echoes:
// the name they set on their profile, or the part of their email before the @.
func DisplayName(u *pb.User) string {
	if u.GetName() != "" {
		return u.GetName()
	}
	local, _, _ := strings.Cut(u.GetEmail(), "@")
	return local
}

// displayNameFor is DisplayName for the user with this email.
func displayNameFor(email string) string {
	u, _ := be.GetUserByEmail(email)
	if u == nil {
		u = &pb.User{Email: email}
	}
	return DisplayName(u)
}

// SetUserName saves the name u is shown as; empty clears it. Names go into
// console lines and logs, so they're one line of up to 32 characters.
// shortcut: users aren't locked, so a page rendering u while it's renamed
// could race; add a users lock if renames become frequent.
func SetUserName(u *pb.User, name string) error {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 32 || strings.ContainsFunc(name, unicode.IsControl) {
		return errors.New("names are up to 32 characters on one line")
	}
	if _, err := db.Handle.Exec("UPDATE user SET name = ? WHERE email = ?", name, u.GetEmail()); err != nil {
		log.Printf("error saving %q's name: %v\n", u.GetEmail(), err)
		return errors.New("unable to save your name, try again")
	}
	u.Name = name
	return nil
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
	stats, err := loadStats(out.Frontends, span)
	out.Stats = stats
	return out, err
}

// Displays info page for a particular client
func WebsiteHandlerServerView(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	renderServerView(w, user, fe, notices[r.URL.Query().Get("notice")], "", r.URL.Query().Get("span"))
}

// renderServerView shows fe's page, optionally with a notice from a previous
// action or the reason a settings change was rejected.
func renderServerView(w http.ResponseWriter, user *pb.User, fe *frontend.Frontend, notice *Notice, formError, span string) {
	data := PageResponse{SessionUser: user, Frontend: fe, Notice: notice, FormError: formError}
	fe.Mu.Lock()
	data.CanEdit = strings.EqualFold(fe.Owner, user.GetEmail()) // delegates can't change settings
	fe.Mu.Unlock()
	var err error
	if data.Stats, err = loadStats([]*frontend.Frontend{fe}, span); err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to load stats", http.StatusInternalServerError)
		return
	}
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
	"map":          {"success", "Map change sent. The current map updates once the server reports it."},
	"badmap":       {"danger", "Map names can only contain letters, numbers, dashes, underscores and slashes."},
	"offline":      {"warning", "That needs the server to be connected."},
	"saved":        {"success", "Settings saved."},
	"uuid":         {"success", "New UUID generated. Update the server's q2admin config to use it."},
	"key":          {"success", "Public key saved."},
	"badkey":       {"danger", "That isn't a valid PEM-encoded RSA public key."},
	"noplayer":     {"warning", "That player isn't connected any more."},
	"badinput":     {"danger", "Check the message or number of seconds and try again."},
	"sent":         {"success", "Message sent."},
	"kicked":       {"success", "Player kicked."},
	"muted":        {"success", "Player muted."},
	"stifled":      {"success", "Player stifled."},
	"banned":       {"success", "Player banned and kicked. The ban rule is on the Rules tab."},
	"blackholed":   {"success", "Blackhole command sent for the player's address."},
	"rulesaved":    {"success", "Rule saved."},
	"namesaved":    {"success", "Name saved."},
	"ruledeleted":  {"success", "Rule deleted."},
	"delegated":    {"success", "Access granted. The server now shows up in their dashboard."},
	"undelegated":  {"success", "Access removed."},
	"nouser":       {"danger", "Nobody is registered with that email address. They need to sign in once first."},
	"selfdelegate": {"warning", "You already own this server."},
	"groupcreated": {"success", "Group created. Add some servers to it."},
	"groupdeleted": {"success", "Group deleted, with its rules. Its servers weren't otherwise changed."},
	"groupadd":     {"success", "Server added to the group."},
	"groupremove":  {"success", "Server removed from the group."},
}

var mapNameRE = regexp.MustCompile(`^[A-Za-z0-9_/-]{1,64}$`)

// DelegateHandler lets a server's owner give a registered user access to it
// (action=add) or take that access away (action=remove). Delegates can do
// everything except change the server's settings.
func DelegateHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, true)
	if fe == nil {
		return
	}
	email := strings.TrimSpace(r.PostFormValue("email"))
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	notice := ""
	switch r.PostFormValue("action") {
	case "add":
		u, _ := be.GetUserByEmail(email)
		switch {
		case u == nil:
			notice = "nouser"
		case strings.EqualFold(u.GetEmail(), fe.Owner):
			notice = "selfdelegate"
		case fe.CanAccess(u.GetEmail()):
			notice = "delegated" // already has it
		default:
			if err := addDelegate(fe, u.GetEmail()); err != nil {
				log.Println(err)
				http.Error(w, "500 - unable to save", http.StatusInternalServerError)
				return
			}
			notice = "delegated"
		}
	case "remove":
		for _, d := range fe.Delegates {
			if !strings.EqualFold(d, email) {
				continue
			}
			if _, err := db.Handle.Exec("DELETE FROM frontend_delegate WHERE frontend_id = ? AND email = ?", fe.ID, d); err != nil {
				log.Printf("error removing %q's delegate %q: %v\n", fe.Name, d, err)
				http.Error(w, "500 - unable to save", http.StatusInternalServerError)
				return
			}
			fe.Delegates = slices.DeleteFunc(slices.Clone(fe.Delegates), func(x string) bool { return x == d })
		}
		notice = "undelegated"
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, path.Join("/sv", fe.UUID, fe.Name)+"?notice="+notice+"#settings", http.StatusSeeOther)
}

// addDelegate shares fe with a registered user who doesn't already have
// access. Call it holding fe.Mu.
func addDelegate(fe *frontend.Frontend, email string) error {
	if _, err := db.Handle.Exec("INSERT INTO frontend_delegate (frontend_id, email) VALUES (?, ?)", fe.ID, email); err != nil {
		return fmt.Errorf("error delegating %q to %q: %v", fe.Name, email, err)
	}
	fe.Delegates = append(slices.Clone(fe.Delegates), email)
	return nil
}

// ChangeMapHandler tells a connected frontend to switch maps.
func ChangeMapHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
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
	tmpl, e := template.ParseFS(website.Files, "templates/new/sign-in.tmpl")

	var page PageResponse
	for i := range Website.Creds {
		page.AuthProviders = append(page.AuthProviders, AuthProvider{
			URL:     BuildAuthURL(Website.Creds[i], i),
			Icon:    Website.Creds[i].GetImagePath(),
			Alt:     Website.Creds[i].GetAlternateText(),
			Name:    providerName(Website.Creds[i].GetType().String()),
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
	renderInfoPage(w, r, "privacy-policy.tmpl", "privacy-policy", "Privacy Policy | CloudAdmin")
}

func TermsHandler(w http.ResponseWriter, r *http.Request) {
	renderInfoPage(w, r, "terms-of-use.tmpl", "terms-of-use", "Terms of Use | CloudAdmin")
}

// renderInfoPage shows a page that doesn't need a signed-in user, like the
// privacy policy, which the sign-in page links to.
func renderInfoPage(w http.ResponseWriter, r *http.Request, file, name, title string) {
	user, _ := GetSessionUser(r) // nil when signed out
	data := PageResponse{SessionUser: user}
	data.Head.Title = title
	tmpl, err := parsePage(file)
	if err != nil {
		log.Println(err)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Println(err)
	}
}

// PublicKeyHandler serves CloudAdmin's public key, which Quake 2 server admins
// download for their q2admin config. It's encoded from the key loaded at
// startup, so it always matches the private key in use.
func PublicKeyHandler(w http.ResponseWriter, r *http.Request) {
	der, err := x509.MarshalPKIXPublicKey(be.publicKey)
	if err != nil {
		log.Println("error encoding the public key:", err)
		http.Error(w, "500 - public key unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="public.pem"`)
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// ProfileHandler shows the signed-in user's account details (GET), or sets
// their name (POST).
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{SessionUser: user, Notice: notices[r.URL.Query().Get("notice")]}
	if r.Method == http.MethodPost {
		if err := SetUserName(user, r.PostFormValue("name")); err != nil {
			data.FormError, data.Form = err.Error(), r.PostForm
		} else {
			http.Redirect(w, r, "/me?notice=namesaved", http.StatusSeeOther)
			return
		}
	}
	data.Frontends = FrontendsByIdentity(user.GetEmail())
	data.Head.Title = "Profile | CloudAdmin"
	tmpl, err := parsePage("profile.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "profile", data, data.Frontends...); err != nil {
		log.Println(err)
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
		renderServerView(w, user, f, nil, err.Error(), "")
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

// PlayerViewHandler shows a connected player and the controls for them.
func PlayerViewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	data := PageResponse{SessionUser: user, Frontend: fe, Notice: notices[r.URL.Query().Get("notice")]}
	data.NavHighlight.Servers = "active"
	if id, err := strconv.Atoi(mux.Vars(r)["ClientNum"]); err == nil {
		fe.Mu.Lock()
		data.Player, _ = fe.FindPlayer(id) // nil renders "not connected"
		fe.Mu.Unlock()
	}
	data.Head.Title = "Player | CloudAdmin"
	tmpl, err := parsePage("player-view.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "player-view", data, fe); err != nil {
		log.Println(err)
	}
}

// PlayerActionHandler messages, kicks, mutes or stifles a connected player.
func PlayerActionHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["ClientNum"])
	// newlines would break the in-game text, and it's a short chat line
	msg := strings.Join(strings.Fields(r.PostFormValue("msg")), " ")
	if len(msg) > 150 {
		msg = msg[:150]
	}
	secs, secsErr := strconv.Atoi(r.PostFormValue("seconds"))

	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	back := path.Join("/sv", fe.UUID, fe.Name, "player", strconv.Itoa(id))
	p, err := fe.FindPlayer(id)
	if err != nil || !(fe.Connected && fe.Trusted) {
		http.Redirect(w, r, back+"?notice=noplayer", http.StatusSeeOther)
		return
	}
	notice := ""
	switch r.PostFormValue("action") {
	case "message":
		if msg == "" {
			notice = "badinput"
			break
		}
		SayPlayer(fe, p, PRINT_CHAT, msg)
		notice = "sent"
	case "kick":
		KickPlayer(fe, p, msg)
		SendMessages(fe)
		http.Redirect(w, r, path.Join("/sv", fe.UUID, fe.Name)+"?notice=kicked#players", http.StatusSeeOther)
		return
	case "ban":
		rule := &pb.Rule{
			Uuid:        uuid.NewString(),
			Type:        pb.RuleType_BAN,
			Scope:       "local",
			Address:     []string{p.IP + "/32"},
			Description: []string{fmt.Sprintf("%s banned by %s from the website", p.Name, user.GetEmail())},
		}
		if msg != "" {
			rule.Message = []string{msg}
		}
		if net.ParseIP(p.IP).To4() == nil {
			rule.Address = []string{p.IP + "/128"}
		}
		if err := fe.InsertRule(rule); err != nil {
			log.Println(err)
			http.Error(w, "500 - unable to save the ban rule", http.StatusInternalServerError)
			return
		}
		fe.Rules = append(fe.Rules, rule)
		KickPlayer(fe, p, msg)
		SendMessages(fe)
		http.Redirect(w, r, path.Join("/sv", fe.UUID, fe.Name)+"?notice=banned#players", http.StatusSeeOther)
		return
	case "blackhole":
		ip := net.ParseIP(p.IP)
		if ip == nil { // it goes out as a console command
			notice = "badinput"
			break
		}
		ConsoleCommand(fe, "addblackhole "+ip.String())
		notice = "blackholed"
	case "mute":
		if secsErr != nil || secs < 0 || secs > 86400 {
			notice = "badinput"
			break
		}
		MutePlayer(fe, p, secs) // 0 is permanent
		notice = "muted"
	case "stifle":
		if secsErr != nil || secs < 1 || secs > StifleMax {
			notice = "badinput"
			break
		}
		StiflePlayer(fe, p, secs)
		notice = "stifled"
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	SendMessages(fe)
	http.Redirect(w, r, back+"?notice="+notice, http.StatusSeeOther)
}

// ServerConsoleHandler shows the live console page for a frontend.
func ServerConsoleHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
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

// frontendFor finds the frontend named in the URL and checks user may use it:
// as its owner, or as a delegate unless ownerOnly (settings). It writes the
// error response and returns nil if not.
func frontendFor(w http.ResponseWriter, r *http.Request, user *pb.User, ownerOnly bool) *frontend.Frontend {
	fe, err := be.FindFrontend(mux.Vars(r)["ServerUUID"])
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	fe.Mu.Lock()
	access := fe.CanAccess(user.GetEmail())
	owner := strings.EqualFold(fe.Owner, user.GetEmail())
	fe.Mu.Unlock()
	if !access {
		http.NotFound(w, r) // same as a server that doesn't exist
		return nil
	}
	if ownerOnly && !owner {
		http.Error(w, "403 - only the server's owner can change its settings", http.StatusForbidden)
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
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the server's WriteTimeout would cut the stream off
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // tell nginx not to buffer this response

	stream := make(chan frontend.ConsoleLine, 64) // ConsoleBroadcast drops messages when this is full
	// a reconnecting browser sends the id of the last line it got, so it's
	// only sent what it missed; a newly opened console gets the whole backlog
	after, _ := strconv.ParseUint(r.Header.Get("Last-Event-ID"), 10, 64)
	backlog := fe.AddTerminal(&stream, after)
	defer fe.TerminalDisconnected(&stream)
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	send := func(line frontend.ConsoleLine) {
		data, _ := json.Marshal(line) // one line, whatever the text contains
		fmt.Fprintf(w, "id: %d\ndata: %s\n\n", line.Seq, data)
	}
	fmt.Fprint(w, ": connected\n\n")
	for _, line := range backlog {
		send(line)
	}
	for {
		if err := rc.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
		case line := <-stream:
			send(line)
		}
	}
}

// validConsoleCommand checks a command typed by a user before it's sent to a
// server console. A newline would smuggle in extra commands.
func validConsoleCommand(cmd string) error {
	if cmd == "" || len(cmd) > 256 || strings.ContainsAny(cmd, "\r\n") {
		return errors.New("send one command of up to 256 characters")
	}
	return nil
}

// ConsoleCommandHandler runs one console command on a frontend, as if typed
// at its server console. Output arrives through ConsoleStreamHandler.
func ConsoleCommandHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		http.Error(w, "401 - signed out", http.StatusUnauthorized)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	cmd := LowerCommandName(strings.TrimSpace(r.PostFormValue("cmd"))) // so the echo shows what runs
	if err := validConsoleCommand(cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	if !(fe.Connected && fe.Trusted) {
		http.Error(w, "the server is offline", http.StatusConflict)
		return
	}
	fe.ConsoleBroadcast(fmt.Sprintf("[%s] > %s", DisplayName(user), cmd)) // other watchers see it too, before its output
	ConsoleCommand(fe, cmd)
	SendMessages(fe)
	fe.Log.Printf("web console [%s]: %s\n", user.GetEmail(), cmd)
	w.WriteHeader(http.StatusNoContent)
}

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

// RuleListHandler lists a frontend's rules.
func RuleListHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	data := PageResponse{SessionUser: user, Frontend: fe, Notice: notices[r.URL.Query().Get("notice")]}
	data.Head.Title = "Rules | CloudAdmin"
	data.NavHighlight.Servers = "active"
	groupRules, err := fe.FetchGroupRules()
	if err != nil {
		log.Println(err)
	}
	data.GroupRules = SortRules(groupRules)
	fe.Mu.Lock()
	data.Rules = SortRules(fe.Rules) // a sorted copy, fe.Rules keeps its order
	data.RuleHome = serverHome(fe)
	fe.Mu.Unlock()
	renderRulePage(w, "rule-list.tmpl", "rule-list", data, data.RuleHome)
}

// RuleHome is what a rule belongs to: a server or a server group. The rule
// pages use it for their links.
type RuleHome struct {
	Kind     string // "server" or "group"
	Name     string
	URL      string // the server's or group's page
	RulesURL string // where its rules are listed
	AddURL   string // the new-rule form
	Root     string // the list it's in, "Servers" or "Groups"
	RootURL  string
	fe       *frontend.Frontend
	group    *ServerGroup
}

// serverHome is the RuleHome for fe. Call it holding fe.Mu.
func serverHome(fe *frontend.Frontend) *RuleHome {
	base := path.Join("/sv", fe.UUID, fe.Name)
	return &RuleHome{Kind: "server", Name: fe.Name, URL: base, RulesURL: base + "/rules", AddURL: base + "/rules/add",
		Root: "Servers", RootURL: "/my-servers", fe: fe}
}

func groupHome(g *ServerGroup) *RuleHome {
	base := "/groups/" + g.Name
	return &RuleHome{Kind: "group", Name: g.Name, URL: base, RulesURL: base + "#rules", AddURL: base + "/rules/add",
		Root: "Groups", RootURL: "/groups", group: g}
}

// set is where the home's rules are stored.
func (h *RuleHome) set() frontend.RuleSet {
	if h.group != nil {
		return frontend.RuleSet{DB: &db, Group: h.group.ID}
	}
	return frontend.RuleSet{DB: h.fe.Data, Frontend: h.fe.ID}
}

// RuleViewHandler shows one of the user's rules.
func RuleViewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	home, rule := findMyRule(user, mux.Vars(r)["uuid"])
	if rule == nil {
		http.NotFound(w, r)
		return
	}
	data := PageResponse{SessionUser: user, Rule: rule, RuleHome: home, Notice: notices[r.URL.Query().Get("notice")]}
	data.Head.Title = "Rule | CloudAdmin"
	renderRulePage(w, "rule-view.tmpl", "rule", data, home)
}

// RuleAddHandler shows the new-rule form for a frontend and creates the rule.
func RuleAddHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	fe.Mu.Lock()
	home := serverHome(fe)
	fe.Mu.Unlock()
	ruleForm(w, r, user, home, newRule(), func(rule *pb.Rule) error {
		fe.Mu.Lock()
		defer fe.Mu.Unlock()
		if err := fe.InsertRule(rule); err != nil {
			return err
		}
		fe.Rules = append(fe.Rules, rule)
		return nil
	})
}

// GroupRuleAddHandler shows the new-rule form for one of the user's groups and
// creates the rule, which then applies to every server in the group.
func GroupRuleAddHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	g, err := MyGroup(user.GetEmail(), mux.Vars(r)["group"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	home := groupHome(g)
	ruleForm(w, r, user, home, newRule(), home.set().Insert)
}

// newRule is a new rule as the form first shows it.
func newRule() *pb.Rule {
	return &pb.Rule{Uuid: uuid.NewString(), Scope: "local", Type: pb.RuleType_BAN, StifleLength: 60}
}

// ruleForm shows the new or edit rule form for rule; on POST it fills rule
// in from the form and saves it with save, then shows the saved rule.
func ruleForm(w http.ResponseWriter, r *http.Request, user *pb.User, home *RuleHome, rule *pb.Rule, save func(*pb.Rule) error) {
	data := PageResponse{SessionUser: user, Rule: rule, RuleHome: home}
	data.Head.Title = "Rule | CloudAdmin"
	if r.Method == http.MethodPost {
		if err := ruleFromForm(r, rule); err != nil {
			data.FormError = err.Error()
		} else if err := save(rule); err != nil {
			log.Println(err)
			data.FormError = "unable to save the rule, try again"
		} else {
			http.Redirect(w, r, "/rules/"+rule.Uuid+"/view?notice=rulesaved", http.StatusSeeOther)
			return
		}
	}
	renderRulePage(w, "rule-edit.tmpl", "rule-edit", data, home)
}

// RuleEditHandler shows the edit form for one of the user's rules and saves it.
func RuleEditHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	home, rule := findMyRule(user, mux.Vars(r)["uuid"])
	if rule == nil {
		http.NotFound(w, r)
		return
	}
	fe := home.fe
	if fe == nil { // a group's rule: read fresh from the database, nothing to keep in sync
		ruleForm(w, r, user, home, rule, home.set().Update)
		return
	}
	fe.Mu.Lock()
	edited := proto.Clone(rule).(*pb.Rule) // fe.Rules is only changed once saved
	fe.Mu.Unlock()
	ruleForm(w, r, user, home, edited, func(edited *pb.Rule) error {
		fe.Mu.Lock()
		defer fe.Mu.Unlock()
		if err := fe.UpdateRule(edited); err != nil {
			return err
		}
		for i := range fe.Rules {
			if fe.Rules[i] == rule {
				fe.Rules[i] = edited
			}
		}
		return nil
	})
}

// RuleDeleteHandler removes one of the user's rules.
func RuleDeleteHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	home, rule := findMyRule(user, mux.Vars(r)["uuid"])
	if rule == nil {
		http.NotFound(w, r)
		return
	}
	if fe := home.fe; fe != nil {
		fe.Mu.Lock()
		defer fe.Mu.Unlock()
	}
	if err := home.set().Delete(rule.GetUuid()); err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to delete the rule", http.StatusInternalServerError)
		return
	}
	if fe := home.fe; fe != nil {
		fe.Rules = slices.DeleteFunc(slices.Clone(fe.Rules), func(x *pb.Rule) bool { return x == rule })
	}
	back := home.URL + "?notice=ruledeleted#rules"
	if home.fe != nil {
		back = home.RulesURL + "?notice=ruledeleted"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// findMyRule finds the rule with this UUID among the user's servers, then
// among their groups' rules.
func findMyRule(user *pb.User, id string) (*RuleHome, *pb.Rule) {
	for _, fe := range FrontendsByIdentity(user.GetEmail()) {
		fe.Mu.Lock()
		for _, rule := range fe.Rules {
			if rule.GetUuid() == id {
				home := serverHome(fe)
				fe.Mu.Unlock()
				return home, rule
			}
		}
		fe.Mu.Unlock()
	}
	groups, err := MyGroups(user.GetEmail())
	if err != nil {
		log.Println(err)
	}
	for _, g := range groups {
		rules, err := frontend.RuleSet{DB: &db, Group: g.ID}.Fetch()
		if err != nil {
			log.Println(err)
		}
		for _, rule := range rules {
			if rule.GetUuid() == id {
				return groupHome(g), rule
			}
		}
	}
	return nil, nil
}

// renderRulePage renders a rule page, holding the server's lock for a server's
// rule since the page shows its rules.
func renderRulePage(w http.ResponseWriter, file, name string, data PageResponse, home *RuleHome) {
	if home.Kind == "server" {
		data.NavHighlight.Servers = "active"
	} else {
		data.NavHighlight.Groups = "active"
	}
	tmpl, err := parsePage(file)
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, name, data, home.fe); err != nil {
		log.Println(err)
	}
}

// ruleFromForm copies the rule form into rule, then validates it. rule is
// filled in even when invalid, so the form can be shown again as entered.
// Fields the form doesn't cover (UUID, timespec, exceptions) are left alone.
func ruleFromForm(r *http.Request, rule *pb.Rule) error {
	lines := func(field string) []string { return splitFormLines(r.PostFormValue(field)) }
	typ, typeOK := pb.RuleType_value[strings.ToUpper(r.PostFormValue("type"))]
	rule.Type = pb.RuleType(typ)
	rule.Description = lines("description")
	rule.Address = lines("addresses")
	rule.Name = lines("names")
	rule.Hostname = lines("hostnames")
	rule.Client = lines("clients")
	rule.Message = lines("messages")
	rule.Vpn = r.PostFormValue("vpn") == "on"
	rule.Disabled = r.PostFormValue("disabled") == "on"
	rule.UserInfo = nil
	for _, line := range lines("userinfo") {
		k, v, _ := strings.Cut(line, "=")
		rule.UserInfo = append(rule.UserInfo, &pb.UserInfo{Property: strings.TrimSpace(k), Value: strings.TrimSpace(v)})
	}
	stifle, stifleErr := strconv.Atoi(r.PostFormValue("stifle_length"))
	if stifleErr == nil {
		rule.StifleLength = int32(stifle)
	}
	switch expires := r.PostFormValue("expires"); expires {
	case "keep":
	case "", "never":
		rule.ExpirationTime = 0
	default:
		secs, err := strconv.ParseInt(expires, 10, 64)
		if err != nil || secs <= 0 {
			return errors.New("pick when the rule expires")
		}
		rule.ExpirationTime = time.Now().Unix() + secs
	}

	if !typeOK {
		return errors.New("pick a rule type")
	}
	for i, a := range rule.Address {
		if ip := net.ParseIP(a); ip != nil { // a single address
			if ip.To4() != nil {
				rule.Address[i] = a + "/32"
			} else {
				rule.Address[i] = a + "/128"
			}
		} else if _, _, err := net.ParseCIDR(a); err != nil {
			return fmt.Errorf("%q isn't an IP address or CIDR network", a)
		}
	}
	for _, list := range [][]string{rule.Name, rule.Hostname, rule.Client} {
		for _, re := range list {
			if _, err := regexp.Compile(re); err != nil {
				return fmt.Errorf("%q isn't a valid regular expression", re)
			}
		}
	}
	for _, ui := range rule.UserInfo {
		if ui.Property == "" {
			return errors.New("userinfo lines need to look like property=pattern")
		}
		if _, err := regexp.Compile(ui.Value); err != nil {
			return fmt.Errorf("%q isn't a valid regular expression", ui.Value)
		}
	}
	if len(rule.Address)+len(rule.Name)+len(rule.Hostname)+len(rule.Client)+len(rule.UserInfo) == 0 && !rule.Vpn {
		return errors.New("add at least one thing to match: an address, name, hostname, client, userinfo or VPN")
	}
	if rule.Type == pb.RuleType_MESSAGE && len(rule.Message) == 0 {
		return errors.New("message rules need a message to show")
	}
	if rule.Type == pb.RuleType_STIFLE && (stifleErr != nil || stifle < 1 || stifle > StifleMax) {
		return fmt.Errorf("stifle length must be 1 to %d seconds", StifleMax)
	}
	return nil
}

// splitFormLines splits a textarea into its non-blank, trimmed lines.
func splitFormLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ServerKeysHandler shows the key page for a frontend, generates a key pair
// to show once (action=Generate), or saves a public key (action=SaveKey).
func ServerKeysHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, true) // keys are settings
	if fe == nil {
		return
	}
	data := PageResponse{SessionUser: user, Frontend: fe}
	data.Head.Title = "Keys | CloudAdmin"
	data.NavHighlight.Servers = "active"

	switch r.PostFormValue("action") {
	case "SaveKey":
		keydata := strings.TrimSpace(r.PostFormValue("keydata"))
		fe.Mu.Lock()
		defer fe.Mu.Unlock()
		back := path.Join("/sv", fe.UUID, fe.Name)
		if _, err := crypto.LoadPublicKey([]byte(keydata)); err != nil {
			http.Redirect(w, r, back+"?notice=badkey#settings", http.StatusSeeOther)
			return
		}
		if _, err := db.Handle.Exec("UPDATE frontend SET public_key_data=? WHERE id=?", keydata, fe.ID); err != nil {
			log.Printf("error saving public key for %q: %v\n", fe.Name, err)
			http.Error(w, "500 - unable to save the key", http.StatusInternalServerError)
			return
		}
		fe.PublicKeyData = keydata
		http.Redirect(w, r, back+"?notice=key#settings", http.StatusSeeOther)
		return
	case "Generate":
		priv, pub, err := crypto.GenerateKeyPair(2048)
		if err != nil {
			log.Println(err)
			http.Error(w, "500 - key generation failed", http.StatusInternalServerError)
			return
		}
		data.KeyPair = &KeyPair{Private: string(priv), Public: string(pub)}
		w.Header().Set("Cache-Control", "no-store") // the private key is only ever shown here
	}

	tmpl, err := parsePage("generate-keys.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "keygen", data, fe); err != nil {
		log.Println(err)
	}
}

// KeyPair is a freshly generated key pair, PEM-encoded. It's shown once and
// never stored.
type KeyPair struct {
	Private, Public string
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

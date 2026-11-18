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
	"fmt"
	"github.com/packetflinger/q2admind/api/website"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/frontend"
	"github.com/packetflinger/q2admind/util"

	pb "github.com/packetflinger/q2admind/proto"
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
		"versions":   softwareVersions,
		"join":       strings.Join,
		"userinfo":   frontend.UserinfoMap,
		"lines":      func(l []string) string { return strings.Join(l, "\n") },
		"uilines":    userinfoLines,
		"ipaddr":     splitIPAddr,
		"iplens":     ipLensURL,
		"spans":      func() any { return StatSpans },
		"duration":   playDuration,
		"avg":        formatAvg,
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
	Form          url.Values         // submitted form values, to refill a form after an error
	FormError     string             // why the submitted form was rejected
	Notice        *Notice            // result of the action that redirected here
	Connection    *ConnectionHistory // the server page's up/down chart, nil with no history
	ServerNames   map[string]string  // UUID -> name of the user's servers
	KeyPair       *KeyPair           // just generated, for the keys page
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

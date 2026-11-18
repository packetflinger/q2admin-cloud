// Website pages for rules: listing, viewing, adding, editing and deleting a
// server's or group's rules.

package backend

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
	"google.golang.org/protobuf/proto"
)

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

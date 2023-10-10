package server

import (
	"log"
	"net"
	"os"
	"regexp"
	"time"

	pb "github.com/packetflinger/q2admind/proto"
	"google.golang.org/protobuf/encoding/prototext"
)

// An ACL
//
// Match on network or hostname
// Then any additional criteria:
// - name
// - userinfo value
// - etc
//
// Password field is checked against the "pw" userinfo variable.
// If the password matches, then the rule is considered to be
// not a match.
type ClientRule struct {
	ID           string       // uuid
	Type         string       // ["ban","mute","stifle","msg"]
	Address      []string     // ip/cidrs
	Network      []*net.IPNet // byte version of address
	Hostname     []string     // hostname
	HostAddrNot  bool         // != instead of == for ip/host
	Name         []string     // optional, names to match
	NameNot      bool         // != instead of ==
	Client       []string     // optional, probably remove later
	UserInfoKey  []string     // optional
	UserinfoVal  []string     // optional
	UserInfoNot  []bool       // != instead of ==
	Description  string       // internal only
	Message      string       // message displayed to matched players
	Password     string       // password to bypass this rule
	StifleLength int          // secs
	Created      int64        // unix timestamp
	Length       int64        // secs after Created before expiring. 0 = perm
}

// Disk format for ACLs
type ClientRuleFormat struct {
	ID           string   `json:"ID"`           // UUID
	Type         string   `json:"Type"`         // ["ban","mute","stifle","msg"]
	Address      []string `json:"Address"`      // x.x.x.x/y
	Hostname     []string `json:"Hostname"`     // dns name
	HostAddrNot  bool     `json:"HostAddrNot"`  // != instead of ==
	Name         []string `json:"Name"`         // optional, player names
	NameNot      bool     `json:"NameNot"`      // != instead of ==
	Client       []string `json:"Client"`       // optional, player client versions
	Message      string   `json:"Message"`      // shown to user
	UserInfoKey  []string `json:"UserInfoKey"`  // optional
	UserinfoVal  []string `json:"UserInfoVal"`  // optional
	UserInfoNot  []bool   `json:"UserInfoNot"`  // !=
	Description  string   `json:"Description"`  // internal only
	Insensitive  bool     `json:"Insensitive"`  // case insensitive?
	Password     string   `json:"Password"`     // override userinfo password
	StifleLength int      `json:"StifleLength"` // seconds
	Created      int64    `json:"Created"`      // unix timestamp
	Length       int64    `json:"Length"`       // seconds after created before expires
}

// Check a client against the rules, returns whether there were
// any matches and what specific rules matched, for processing
// later
//
// Called every time a player connects from ApplyRules() in ParseConnect()
func (cl *Client) CheckRules(p *Player, ruleset []*pb.Rule) (bool, []*pb.Rule) {
	rules := []*pb.Rule{} // which ones match
	for _, r := range ruleset {
		if cl.CheckRule(p, r) {
			rules = append(rules, r)
		}
	}

	return len(rules) > 0, rules
}

// Check of a player matches a particular rule
func (cl *Client) CheckRule(p *Player, r *pb.Rule) bool {
	match := false
	now := time.Now().Unix()
	need := 0
	have := 0

	// expired rule, ignore it
	if r.GetExpirationTime() > 0 && now > r.GetExpirationTime() {
		return false
	}

	// if user has the password, the rule will never match
	//if r.Password != "" && p.UserinfoMap["pw"] == r.Password {
	//	return false
	//}

	// any IPs
	if len(r.GetAddress()) > 0 {
		need++
		for _, address := range r.GetAddress() {
			_, network, err := net.ParseCIDR(address)
			if err != nil {
				continue
			}
			if network.Contains(net.ParseIP(p.IP)) {
				have++
				match = true
				break
			}
		}
	}

	/*
		// any hostnames (regex)
		if len(r.Hostname) > 0 {
			need++
			for _, host := range r.Hostname {
				hm, err := regexp.MatchString(host, p.Hostname)
				if err != nil {
					continue
				}
				if hm {
					have++
					match = true
					break
				}
			}
		}
	*/

	if len(r.Name) > 0 {
		need++
		for _, name := range r.Name {
			// case insensitive
			namematch, err := regexp.MatchString("(?i)"+name, p.Name)
			if err != nil {
				continue
			}
			/*
				if r.NameNot {
					if !namematch {
						match = true
						have++
					}
				} else*/{
				if namematch {
					match = true
					have++
				}
			}
		}
	}

	// userinfo stuff, all have to match
	//uinot := false
	if len(r.GetUserInfo()) > 0 {
		for _, ui := range r.GetUserInfo() {
			need++
			if UserinfoMatches(ui, p) {
				have++
			}
		}
	}

	return match && (need <= have)
}

/*
// Player should already match each rule, just apply the action.
//
// Called immediately after CheckRules() on ParseConnect() twice,
// for local server rules and then again for global ones
func (cl *Client) ApplyRules(p *Player) {
	matched1, rules1 := cl.CheckRules(p, cl.Rules)  // local
	matched2, rules2 := cl.CheckRules(p, q2a.rules) // global

	if matched1 {
		for _, r := range rules1 {
			log.Printf("%s [%d|%s] matched global rule %s\n", p.Name, p.ClientID, p.IP, r.ID)
			switch r.Type {
			case "msg":
				log.Printf("[%s/MSG/%s] %s\n", cl.Name, p.Name, r.Message)
				SayPlayer(cl, p, PRINT_MEDIUM, r.Message)
			case "ban":
				log.Printf("[%s/KICK/%s] %s\n", cl.Name, p.Name, r.Message)
				SayPlayer(cl, p, PRINT_MEDIUM, r.Message)
				KickPlayer(cl, p, r.Message)
				return
			case "mute":
				log.Printf("[%s/MUTE/%s] %s\n", cl.Name, p.Name, r.Message)
				SayPlayer(cl, p, PRINT_MEDIUM, r.Message)
				MutePlayer(cl, p, -1)
			case "stifle":
				p.Stifled = true
				p.StifleLength = r.StifleLength
				log.Printf("[%s/STIFLE/%s] %s\n", cl.Name, p.Name, r.Message)
				SayPlayer(cl, p, PRINT_MEDIUM, r.Message)
				MutePlayer(cl, p, r.StifleLength)
			}
		}
	}

	if matched2 {
		for _, r := range rules2 {
			log.Printf("%s [%d|%s] matched global rule %s\n", p.Name, p.ClientID, p.IP, r.ID)
			switch r.Type {
			case "msg":
				log.Printf("[%s/MSG/%s] %s\n", cl.Name, p.Name, r.Message)
				SayPlayer(cl, p, PRINT_MEDIUM, r.Message)
			case "ban":
				log.Printf("[%s/KICK/%s] %s\n", cl.Name, p.Name, r.Message)
				SayPlayer(cl, p, PRINT_MEDIUM, r.Message)
				KickPlayer(cl, p, r.Message)
				return
			case "mute":
				log.Printf("[%s/MUTE/%s] %s\n", cl.Name, p.Name, r.Message)
				SayPlayer(cl, p, PRINT_MEDIUM, r.Message)
				MutePlayer(cl, p, -1)
			case "stifle":
				p.Stifled = true
				p.StifleLength = r.StifleLength
				log.Printf("[%s/STIFLE/%s] %s\n", cl.Name, p.Name, r.Message)
				SayPlayer(cl, p, PRINT_MEDIUM, r.Message)
				MutePlayer(cl, p, r.StifleLength)
			}
		}
	}
}
*/

// Reads and parses the global rules from disk into memory.
//
// Called once at startup
func (q2a *RemoteAdminServer) ReadGlobalRules() {
	filedata, err := os.ReadFile("rules.q2a")
	if err != nil {
		log.Println("problems parsing rules.")
		return
	}

	rules := &pb.Rules{}
	err = prototext.Unmarshal(filedata, rules)
	if err != nil {
		log.Println()
	}
	q2a.rules = SortRules(rules.GetRule())
}

// Read rules from disk
func FetchRules(filename string) ([]*pb.Rule, error) {
	r := []*pb.Rule{}
	filedata, err := os.ReadFile(filename)
	if err != nil {
		return r, err
	}
	rules := &pb.Rules{}
	err = prototext.Unmarshal(filedata, rules)
	if err != nil {
		return r, err
	}
	return rules.GetRule(), nil
}

// Put ban rules first for fast failing.
// Order:
// 1. Bans
// 2. Mutes
// 3. Stifles
// 4. Messages
//
// Called from ReadGlobalRules() and LoadClients() on startup.
// Also called as new rules are added while running
func SortRules(rules []*pb.Rule) []*pb.Rule {
	newruleset := []*pb.Rule{}
	bans := []*pb.Rule{}
	mutes := []*pb.Rule{}
	stifles := []*pb.Rule{}
	msgs := []*pb.Rule{}

	for _, r := range rules {
		switch r.GetType() {
		case pb.RuleType_BAN:
			bans = append(bans, r)
		case pb.RuleType_MUTE:
			mutes = append(mutes, r)
		case pb.RuleType_STIFLE:
			stifles = append(stifles, r)
		case pb.RuleType_MESSAGE:
			msgs = append(msgs, r)
		}
	}

	newruleset = append(newruleset, bans...)
	newruleset = append(newruleset, mutes...)
	newruleset = append(newruleset, stifles...)
	newruleset = append(newruleset, msgs...)
	return newruleset
}

// Transform a ClientRule into the format necessary
// to write it to disk
func (r ClientRule) ToDiskFormat() ClientRuleFormat {
	return ClientRuleFormat{
		ID:           r.ID,
		Type:         r.Type,
		Address:      r.Address,
		Hostname:     r.Hostname,
		HostAddrNot:  r.HostAddrNot,
		Name:         r.Name,
		NameNot:      r.NameNot,
		Client:       r.Client,
		UserInfoKey:  r.UserInfoKey,
		UserinfoVal:  r.UserinfoVal,
		UserInfoNot:  r.UserInfoNot,
		Description:  r.Description,
		Message:      r.Message,
		Password:     r.Password,
		StifleLength: r.StifleLength,
		Created:      r.Created,
		Length:       r.Length,
	}
}

func UserinfoMatches(ui *pb.UserInfo, p *Player) bool {
	for k, v := range p.UserinfoMap {
		if k == ui.GetProperty() {
			re, err := regexp.Compile(ui.GetValue())
			if err != nil {
				return false
			}
			if re.MatchString(v) {
				return true
			}
		}
	}
	return false
}

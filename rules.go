package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
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
func (cl *Client) CheckRules(p *Player, ruleset []ClientRule) (bool, []ClientRule) {
	rules := []ClientRule{} // which ones match
	for _, r := range ruleset {
		if cl.CheckRule(p, r) {
			rules = append(rules, r)
		}
	}

	return len(rules) > 0, rules
}

// Check of a player matches a particular rule
func (cl *Client) CheckRule(p *Player, r ClientRule) bool {
	match := false
	now := time.Now().Unix()
	need := 0
	have := 0

	// expired rule, ignore it
	if r.Length > 0 && now-r.Created > r.Length {
		return false
	}

	// if user has the password, the rule will never match
	if r.Password != "" && p.UserinfoMap["pw"] == r.Password {
		return false
	}

	// any IPs
	if len(r.Network) > 0 {
		need++
		for _, network := range r.Network {
			if network.Contains(net.ParseIP(p.IP)) {
				have++
				match = true
				break
			}
		}
	}

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

	if len(r.Name) > 0 {
		need++
		for _, name := range r.Name {
			// case insensitive
			namematch, err := regexp.MatchString("(?i)"+name, p.Name)
			if err != nil {
				continue
			}
			if r.NameNot {
				if !namematch {
					match = true
					have++
				}
			} else {
				if namematch {
					match = true
					have++
				}
			}
		}
	}

	// userinfo stuff, all have to match
	uinot := false
	if len(r.UserInfoKey) > 0 {
		for i, k := range r.UserInfoKey {
			need++
			if len(r.UserInfoNot) >= i && len(r.UserInfoNot) != 0 {
				uinot = r.UserInfoNot[i]
			} else {
				uinot = false
			}
			if uinot {
				if p.UserinfoMap[k] != r.UserinfoVal[i] {
					match = true
					have++
				}
			} else {
				if p.UserinfoMap[k] == r.UserinfoVal[i] {
					match = true
					have++
				}
			}
		}
	}

	return match && (need <= have)
}

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
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
			case "ban":
				log.Printf("[%s/KICK/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.KickPlayer(p, r.Message)
				return
			case "mute":
				log.Printf("[%s/MUTE/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.MutePlayer(p, -1)
			case "stifle":
				p.Stifled = true
				p.StifleLength = r.StifleLength
				log.Printf("[%s/STIFLE/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.MutePlayer(p, r.StifleLength)
			}
		}
	}

	if matched2 {
		for _, r := range rules2 {
			log.Printf("%s [%d|%s] matched global rule %s\n", p.Name, p.ClientID, p.IP, r.ID)
			switch r.Type {
			case "msg":
				log.Printf("[%s/MSG/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
			case "ban":
				log.Printf("[%s/KICK/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.KickPlayer(p, r.Message)
				return
			case "mute":
				log.Printf("[%s/MUTE/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.MutePlayer(p, -1)
			case "stifle":
				p.Stifled = true
				p.StifleLength = r.StifleLength
				log.Printf("[%s/STIFLE/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.MutePlayer(p, r.StifleLength)
			}
		}
	}
}

// Reads and parses the global rules from disk into memory.
//
// Called once at startup
func (q2a *RemoteAdminServer) ReadGlobalRules() {
	filedata, err := os.ReadFile("rules.json")
	if err != nil {
		log.Println("problems parsing rules.json")
		return
	}

	in := []ClientRuleFormat{}
	rules := []ClientRule{}

	err = json.Unmarshal([]byte(filedata), &in)
	if err != nil {
		log.Println(err)
		return
	}

	for _, r := range in {
		out := ClientRule{}
		out.ID = r.ID
		out.Address = r.Address
		out.Hostname = r.Hostname
		out.HostAddrNot = r.HostAddrNot
		out.Client = r.Client
		out.Created = r.Created
		out.Description = r.Description
		out.Length = r.Length
		out.Message = r.Message
		out.Name = r.Name
		out.NameNot = r.NameNot
		out.Password = r.Password
		out.StifleLength = r.StifleLength
		out.Type = r.Type
		out.UserInfoKey = r.UserInfoKey
		out.UserinfoVal = r.UserinfoVal
		out.UserInfoNot = r.UserInfoNot

		for _, ip := range r.Address {
			if !strings.Contains(ip, "/") { // no cidr notation, assuming /32
				ip += "/32"
			}
			_, netbinary, err := net.ParseCIDR(ip)
			if err != nil {
				log.Println("invalid cidr network in global rule", r.ID, ip)
				continue
			}
			out.Network = append(out.Network, netbinary)
		}
		rules = append(rules, out)
	}

	q2a.rules = SortRules(rules)
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
func SortRules(rules []ClientRule) []ClientRule {
	newruleset := []ClientRule{}
	bans := []ClientRule{}
	mutes := []ClientRule{}
	stifles := []ClientRule{}
	msgs := []ClientRule{}

	for _, r := range rules {
		switch r.Type {
		case "ban":
			bans = append(bans, r)
		case "mute":
			mutes = append(mutes, r)
		case "stifle":
			stifles = append(stifles, r)
		case "msg":
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

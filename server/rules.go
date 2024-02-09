package server

import (
	"net"
	"os"
	"regexp"
	"time"

	"github.com/packetflinger/q2admind/client"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// Check a client against the rules, returns whether there were
// any matches and what specific rules matched, for processing
// later
//
// Called every time a player connects from ApplyRules() in ParseConnect()
func CheckRules(cl *client.Client, p *client.Player, ruleset []*pb.Rule) (bool, []*pb.Rule) {
	rules := []*pb.Rule{} // which ones match
	for _, r := range ruleset {
		if CheckRule(cl, p, r) {
			rules = append(rules, r)
		}
	}

	return len(rules) > 0, rules
}

// Check of a player matches a particular rule
func CheckRule(cl *client.Client, p *client.Player, r *pb.Rule) bool {
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

// Does a player's userinfo match the rules?
//
// Called from CheckRule()
func UserinfoMatches(ui *pb.UserInfo, p *client.Player) bool {
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

// RuleExcptionMatch will decide if a player struct matches a particular
// exception, therefore causing the parent rule to not match this player
// when it otherwise would have.
func RuleExceptionMatch(ex *pb.Exception, p *client.Player) bool {
	now := time.Now().Unix()

	// expired exception, ignore it
	if ex.GetExpirationTime() > 0 && now > ex.GetExpirationTime() {
		return false
	}

	if len(ex.GetAddress()) > 0 {
		for _, address := range ex.GetAddress() {
			_, network, err := net.ParseCIDR(address)
			if err != nil {
				continue
			}
			if network.Contains(net.ParseIP(p.IP)) {
				return true
			}
		}
	}

	if len(ex.GetName()) > 0 {
		for _, name := range ex.GetName() {
			match, err := regexp.MatchString(name, p.Name)
			if err != nil {
				continue
			}
			if match {
				return true
			}
		}
	}

	if len(ex.GetUserInfo()) > 0 {
		for _, uipair := range ex.GetUserInfo() {
			match, err := regexp.MatchString(uipair.Value, p.UserinfoMap[uipair.Property])
			if err != nil {
				continue
			}
			if match {
				return true
			}
		}
	}
	return false
}

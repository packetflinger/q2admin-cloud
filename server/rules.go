package server

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
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
func CheckRules(p *client.Player, ruleset []*pb.Rule) (bool, []*pb.Rule) {
	rules := []*pb.Rule{} // which ones match
	for _, r := range ruleset {
		if CheckRule(p, r) {
			rules = append(rules, r)
		}
	}

	return len(rules) > 0, SortRules(rules)
}

// Check if a player matches a particular rule.
//
// Rule matching is GREEDY, so if multiple critera are specified in the rule,
// all of them have to match, not just any single one.
func CheckRule(p *client.Player, r *pb.Rule) bool {
	match := false
	now := time.Now().Unix()
	need := 0
	have := 0

	// expired rule, ignore it
	if r.GetExpirationTime() > 0 && now > r.GetExpirationTime() {
		return false
	}

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

	if len(r.Hostname) > 0 {
		need++
		for _, host := range r.Hostname {
			// case insensitive
			hm, err := regexp.MatchString("(?i)"+host, p.Hostname)
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
			if namematch {
				match = true
				have++
			}
		}
	}

	// userinfo stuff, all have to match
	if len(r.GetUserInfo()) > 0 {
		originalNeed := need
		originalHave := have
		for _, ui := range r.GetUserInfo() {
			need++
			if UserinfoMatches(ui, p) {
				have++
			}
		}
		if need-originalNeed == have-originalHave {
			match = true
		}
	}

	if r.GetVpn() {
		need++
		if p.VPN {
			have++
			match = true
		}
	}

	exception := false
	for _, ex := range r.GetException() {
		if RuleExceptionMatch(ex, p) {
			exception = true
			break
		}
	}

	applies := match && (need <= have) && !exception

	// set any perisistent data on the user pointer if rule matched
	if match {
		if r.Type == pb.RuleType_STIFLE {
			p.Stifled = true
			p.StifleLength = int(r.GetStifleLength())
		}
		if r.Type == pb.RuleType_MUTE {
			p.Muted = true
		}
	}
	return applies
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
	return SortRules(rules.GetRule()), nil
}

// Put ban rules first for fast failing.
// Order:
// 1. Bans
// 2. Mutes
// 3. Stifles
// 4. Messages
//
// Called from FetchRules() on startup.
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

	if len(ex.GetHostname()) > 0 {
		for _, host := range ex.GetHostname() {
			match, err := regexp.MatchString("(?i)"+host, p.Hostname)
			if err != nil {
				continue
			}
			if match {
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

// durationToSeconds converts a string representation of a duration of time into
// the appropriate number of seconds.
//
// Used in parsing TimeSpec proto messages for rules.
func durationToSeconds(ts string) (int, error) {
	multiplier := map[string]int{
		"s": 1,
		"m": 60,
		"h": 3600,
		"d": 86400,
		"w": 86400 * 7,
		"M": 86400 * 30, // yeah, I know
		"y": 86400 * 7 * 52,
	}
	units := "s"
	trimmed := strings.Trim(ts, " \t\n")

	for u := range multiplier {
		if strings.HasSuffix(trimmed, u) {
			units = u
			trimmed = trimmed[:len(trimmed)-1]
		}
	}
	// it's a decimal
	if strings.Contains(trimmed, ".") {
		value, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, err
		}
		return int(value * float64(multiplier[units])), nil
	}

	// time (clock) notation
	if strings.Contains(trimmed, ":") {
		value := int64(0)
		tokens := strings.Split(trimmed, ":")
		switch len(tokens) {
		case 2: // just minutes and seconds
			tmp, err := strconv.ParseInt(tokens[1], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("can't convert %q to an integer", tokens[1])
			}
			value += tmp
			tmp, err = strconv.ParseInt(tokens[0], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("can't convert %q to an integer", tokens[1])
			}
			value += tmp * int64(multiplier["m"])
			return int(value), nil
		case 3: // hours:minutes:seconds
			tmp, err := strconv.ParseInt(tokens[2], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("can't convert %q to an integer", tokens[1])
			}
			value += tmp
			tmp, err = strconv.ParseInt(tokens[1], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("can't convert %q to an integer", tokens[1])
			}
			value += tmp * int64(multiplier["m"])
			tmp, err = strconv.ParseInt(tokens[0], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("can't convert %q to an integer", tokens[1])
			}
			value += tmp * int64(multiplier["h"])
			return int(value), nil
		}
	}
	value, err := strconv.ParseInt(trimmed, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("can't convert %q to an integer", trimmed)
	}
	return int(value * int64(multiplier[units])), nil
}

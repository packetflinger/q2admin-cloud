package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/packetflinger/q2admind/client"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// Check a client against the rules, returns whether there were
// any matches and what specific rules matched, for processing
// later.
//
// Called every time a player connects from ParseConnect()
func CheckRules(p *client.Player, ruleset []*pb.Rule) (bool, []*pb.Rule) {
	var rules []*pb.Rule
	for _, r := range SortRules(ruleset) {
		if CheckRule(p, r, time.Now()) {
			rules = append(rules, r)
		}
	}
	return len(rules) > 0, rules
}

// Check if a player matches a particular rule.
//
// Rule matching is GREEDY, so if multiple critera are specified in the rule,
// all of them have to match, not just any single one.
//
// Needs and Haves
//
//	Each criteria that is found increases the needs by one. If the player
//	matches that criteria, the haves increase by one. At the end, if the
//	needs and haves are equal (and more than 0), the rule matched the
//	player.
//
// Called from CheckRules()
func CheckRule(p *client.Player, r *pb.Rule, t time.Time) bool {
	match := false
	now := t.Unix()
	need := 0
	have := 0

	if r.GetDisabled() {
		return false
	}

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

	if len(r.GetHostname()) > 0 {
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

	if len(r.GetName()) > 0 {
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

	// timing
	if len(r.GetTimespec().GetBefore()) > 0 {
		need++
		when, err := stringToTime(r.GetTimespec().GetBefore())
		if err != nil {
			p.Client.Log.Printf("error in rule %q, invalid timespec %q\n", r.GetUuid(), r.GetTimespec().GetBefore())
			return false
		}
		// time-only
		if when.Year() == 0 {
			if t.Hour() <= when.Hour() && t.Minute() <= when.Minute() {
				have++
				match = true
			}
		} else {
			if t.Before(when) {
				have++
				match = true
			}
		}
	}

	if len(r.GetTimespec().GetAfter()) > 0 {
		need++
		when, err := stringToTime(r.GetTimespec().GetAfter())
		if err != nil {
			p.Client.Log.Printf("error in rule %q, invalid timespec %q\n", r.GetUuid(), r.GetTimespec().GetAfter())
			return false
		}
		// time-only
		if when.Year() == 0 {
			if t.Hour() >= when.Hour() && t.Minute() >= when.Minute() {
				have++
				match = true
			}
		} else {
			if t.After(when) {
				have++
				match = true
			}
		}
	}

	if len(r.GetTimespec().GetPlayTime()) > 0 {
		need++
		playtime := t.Unix() - p.ConnectTime
		limit, err := durationToSeconds(r.Timespec.GetPlayTime())
		if err != nil {
			p.Client.Log.Printf("error in rule %q, invalid timespec %q\n", r.GetUuid(), r.GetTimespec().GetPlayTime())
			return false
		}
		if playtime >= int64(limit) {
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

// SortRules will reorder the rules in descending seriousness. A player who is
// banned will be kicked, so it doesn't matter if a mute or stifle also match
// them. It's a waste of resources to continue checking less severe rules.
//
// Order:
//
//	Bans > Mutes > Stifles > Messages
//
// Called from FetchRules() on startup.
// Also called as new rules are added while running
func SortRules(rules []*pb.Rule) []*pb.Rule {
	var bans, mutes, stifles, msgs []*pb.Rule
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
	return slices.Concat(bans, mutes, stifles, msgs)
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

// stringToTime will convert a string representation of a date/time to a time
// struct for use when matching rules to clients.
//
// Supported formats:
//
//	"16:30:00" (hour:minute:second)
//	"4:30PM"
//	"2024-10-05" (year-month-day)
//	"2024-10-05 16:30:00" (year-month-day hour:minute:second)
//
// For formats that don't include a date, that time on any day will match. For
// the date-only format, midnight on that day will match.
func stringToTime(t string) (time.Time, error) {
	ts, err := time.Parse(time.TimeOnly, t)
	if err != nil {
		ts, err = time.Parse(time.Kitchen, t)
		if err != nil {
			ts, err = time.Parse(time.DateOnly, t)
			if err != nil {
				ts, err = time.Parse(time.DateTime, t)
				if err != nil {
					return time.Time{}, err
				}
				return ts, nil
			}
			return ts, nil
		}
		return ts, nil
	}
	return ts, nil
}

// Take the appropriate action against the player for the set of rules.
//
// Players are matched against all the rules prior to calling this, so the
// `rules` arg will contain only rules that we know already match
// the player. The set of rules will also already be sorted in descending
// order of severity (bans, mutes, stifles, msgs).
//
// Bans are handled first in order to fast-fail. Once a ban rule is encountered
// the rest of the rules are not processed since that player will be kicked
// from the server immediatly.
func ApplyMatchedRules(p *client.Player, rules []*pb.Rule) {
	if len(rules) == 0 || p == nil {
		return
	}
	cl := p.Client
	cl.Log.Printf("%s|%d matched the following rules:\n", p.Name, p.ClientID)
	for _, rule := range rules {
		cl.Log.Printf("  - %s (%s)\n", strings.Join(rule.GetDescription(), " "), rule.GetType())
	}
	for _, rule := range rules {
		if rule.GetType() == pb.RuleType_BAN {
			KickPlayer(cl, p, strings.Join(rule.Message, "\n"))
			break // don't bother with the rest
		}
		if rule.GetType() == pb.RuleType_MUTE {
			p.Muted = true
			SayPlayer(cl, p, PRINT_CHAT, strings.Join(rule.GetMessage(), " "))
			MutePlayer(cl, p, -1)
		}
		if rule.GetType() == pb.RuleType_STIFLE {
			if p.Muted {
				continue // no point stifling an already muted player
			}
			p.Stifled = true
			p.StifleLength = int(rule.GetStifleLength())
			SayPlayer(cl, p, PRINT_CHAT, "You're stifled")
			MutePlayer(cl, p, p.StifleLength)
		}
		if rule.GetType() == pb.RuleType_MESSAGE {
			SayPlayer(cl, p, PRINT_CHAT, strings.Join(rule.GetMessage(), " "))
		}
	}
}

// RuleDetail will return a condensed string explaining the criteria of
// rule in a single line of text. Depending on the size of the rule
// (the amount of critera and/or exceptions) this could be a long line of
// text.
func RuleDetailLine(rule *pb.Rule) (string, error) {
	if rule == nil {
		return "", errors.New("RuleDetailLine(): empty input")
	}
	out, err := prototext.Marshal(rule)
	if err != nil {
		return "", fmt.Errorf("RuleDetailLine() error: %v", err)
	}
	return string(out), nil
}

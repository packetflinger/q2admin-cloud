package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

// check a client against the rules, returns whether there were
// any matches and what specific rules matched, for processing
func (cl *Client) CheckRules(p *Player, ruleset []ClientRule) (bool, []ClientRule) {
	matched := false        // whether any rules in the set matched
	rules := []ClientRule{} // which ones matches
	need := 0               // the criteria we need to be considered a match
	have := 0               // how many criteria we have
	now := time.Now().Unix()

	for _, r := range ruleset {
		// expired rule
		if now-r.Created > r.Length {
			continue
		}

		// Match the actual IP address
		if r.Network.Contains(net.ParseIP(p.IP)) {
			if p.UserinfoMap["pw"] == r.Password {
				continue
			}
			need++
			have++

			// any one name has to match
			if len(r.Name) > 0 {
				need++
				for _, name := range r.Name {
					if r.Exact {
						if p.Name == name {
							matched = true
							have++
						}
					} else {
						if strings.Contains(p.Name, name) {
							matched = true
							have++
						}
					}
				}
			}

			// userinfo stuff, all have to match
			if len(r.UserInfoKey) > 0 {
				for i, k := range r.UserInfoKey {
					need++
					if r.Exact {
						if p.UserinfoMap[k] == r.UserinfoVal[i] {
							have++
						}
					} else {
						if strings.Contains(p.UserinfoMap[k], r.UserinfoVal[i]) {
							have++
						}
					}
				}
			}

			/* CLIENT STUFF HERE
			if len(r.Client) > 0 {
				reqs_unmet = true
				for _, q2cl := range r.Client {
					if r.Exact {
						if p. == q2cl {
							return true, &cl.Rules[i]
						}
					} else {
						if strings.Contains(p.Name, name) {
							return true, &cl.Rules[i]
						}
					}
				}
			}
			*/

			// if the match is a ban, no point in processing the rest of the rules
			if have == need && r.Type == "ban" {
				rules = append(rules, r)
				return true, rules
			}

			if have == need {
				matched = true
				rules = append(rules, r)
			}
		}
	}

	return matched, rules
}

func (cl *Client) ApplyRules(p *Player) {
	// local rules first
	matched1, rules1 := cl.CheckRules(p, cl.Rules)
	if matched1 {
		for _, r := range rules1 {
			switch r.Type {
			case "msg":
				log.Printf("[%s/MSG/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
			case "ban":
				log.Printf("[%s/KICK/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.KickPlayer(p, r.Message)
			case "mute":
				log.Printf("[%s/MUTE/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.MutePlayer(p, -1)
			}
		}
	}

	// global
	matched2, rules2 := cl.CheckRules(p, cl.Rules)
	if matched2 {
		for _, r := range rules2 {
			switch r.Type {
			case "msg":
				log.Printf("[%s/MSG/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
			case "ban":
				log.Printf("[%s/KICK/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.KickPlayer(p, r.Message)
			case "mute":
				log.Printf("[%s/MUTE/%s] %s\n", cl.Name, p.Name, r.Message)
				cl.SayPlayer(p, PRINT_MEDIUM, r.Message)
				cl.MutePlayer(p, -1)
			}
		}
	}
}

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
		out.Client = r.Client
		out.Created = r.Created
		out.Description = r.Description
		out.Length = r.Length
		out.Message = r.Message
		out.Name = r.Name
		out.Password = r.Password
		out.StifleLength = r.StifleLength
		out.Type = r.Type
		out.UserInfoKey = r.UserInfoKey
		out.UserinfoVal = r.UserinfoVal
		out.Exact = r.Exact
		_, net, err := net.ParseCIDR(r.Address)
		if err == nil {
			out.Network = net
		}
		rules = append(rules, out)
	}

	q2a.rules = rules
}

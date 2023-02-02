package main

import (
	"net"
	"testing"
)

func genrules() []ClientRule {
	rules := []ClientRule{
		{
			ID: "rule1",
			Address: []string{
				"10.1.1.0/25",
				"10.2.3.5/32",
				"10.4.0.0/16",
			},
			Hostname: []string{
				"host1.example.com",
				"host2.example.net",
				"host[a-z]\\.google.com",
			},
			Length: 0,
			Type:   "mute",
		},
		{
			ID: "rule2",
			Address: []string{
				"100.1.2.0/22",
			},
			Length: 0,
			Type:   "ban",
		},
		{
			ID: "rule3",
			Address: []string{
				"24.6.0.0/16",
			},
			Length: 0,
			Type:   "mute",
		},
		{
			ID: "expiredrule1",
			Address: []string{
				"10.1.0.0/16",
			},
			Created: 666,
			Length:  1,
			Type:    "ban",
		},
		{
			ID: "NotName1",
			Address: []string{
				"100.64.1.200/32",
			},
			Name: []string{
				"jimbob",
			},
			NameNot: true,
			Length:  0,
			Type:    "mute",
		},
	}

	for i := range rules {
		for _, ip := range rules[i].Address {
			_, netbin, _ := net.ParseCIDR(ip)
			rules[i].Network = append(rules[i].Network, netbin)
		}
	}

	return rules
}

func TestRuleSort1(t *testing.T) {
	rules := genrules()

	rules2 := SortRules(rules)

	if len(rules) != len(rules2) {
		t.Error("ins and outs don't match")
	}

	if rules2[0].ID != "rule2" {
		t.Error("ban not first", rules2)
	}
}

func TestRuleName1(t *testing.T) {
	rules := genrules()

	q2a.ReadGlobalRules()
	p := Player{
		ClientID: 0,
		Name:     "joe",
		IP:       "10.1.1.1",
	}
	cl := Client{}
	match, mrules := cl.CheckRules(&p, rules)
	if !match {
		t.Error("No match")
	}
	if len(mrules) != 1 {
		t.Error("1 rule should match:", len(mrules), "\n", mrules)
	}
}

func TestRuleExpired1(t *testing.T) {
	rules := genrules()
	p := Player{
		ClientID: 0,
		Name:     "joer",
		IP:       "10.1.1.1",
	}
	cl := Client{}
	_, mrules := cl.CheckRules(&p, rules)
	if len(mrules) != 1 {
		t.Error("0 rule should match:", len(mrules), "\n", mrules)
	}
}

func TestRuleReal1(t *testing.T) {
	rules := genrules()

	p := Player{
		ClientID: 0,
		Name:     "Clairerewe",
		IP:       "10.1.1.1",
		//IP: "24.6.45.55",
		UserinfoMap: map[string]string{
			"pw": "llbean",
		},
	}
	cl := Client{}
	match, mrules := cl.CheckRules(&p, rules)
	if !match {
		t.Error("no match")
	}
	if len(mrules) != 1 {
		t.Error("1 rule should match:", len(mrules), "\n", mrules)
	}
}

func TestRuleUserInfo1(t *testing.T) {
	rules := genrules()

	p := Player{
		ClientID: 0,
		Name:     "claire",
		IP:       "10.1.1.120",
		UserinfoMap: map[string]string{
			"hand": "1",
			"skin": "female/jezebel",
		},
	}
	cl := Client{}
	match, mrules := cl.CheckRules(&p, rules)
	if !match {
		t.Error("no match")
	}
	if len(mrules) != 1 {
		t.Error("1 rule should match:", len(mrules), "\n", mrules)
	}
}

func TestHostname1(t *testing.T) {
	rules := genrules()

	p := Player{
		ClientID:    0,
		Name:        "ostr",
		IP:          "10.200.145.55",
		Hostname:    "vn56.ny.us.hostj.google.com",
		UserinfoMap: map[string]string{
			//"pw": "llbean",
		},
	}

	cl := Client{}
	match, mrules := cl.CheckRules(&p, rules)
	if !match {
		t.Error("no match")
	}
	if len(mrules) != 1 {
		t.Error("1 rule should match:", len(mrules), "\n", mrules)
	}

	if mrules[0].ID != "rule1" {
		t.Error("Not the right rule")
	}
}

func TestNameNot1(t *testing.T) {
	rules := genrules()

	p := Player{
		ClientID: 0,
		Name:     "jimbob",
		IP:       "100.64.1.200",
		//Hostname:    "vn56.ny.us.hostj.google.com",
		UserinfoMap: map[string]string{
			//"pw": "llbean",
		},
	}

	cl := Client{}
	match, mrules := cl.CheckRules(&p, rules)
	if match {
		t.Error("Shouldn't match:", mrules)
	}
}

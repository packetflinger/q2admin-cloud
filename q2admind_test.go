package main

import (
	"net"
	"testing"
)

func TestRules1(t *testing.T) {
	q2a.ReadGlobalRules()
	ui := make(map[string]string)
	ui["pw"] = "armpitfarts"
	p := Player{
		ClientID:    0,
		Name:        "claire",
		IP:          "192.168.3.5",
		UserinfoMap: ui,
	}
	cl := Client{}
	match, rules := cl.CheckRules(&p, q2a.rules)

	if !match {
		t.Errorf("No match")
	} else {
		t.Logf("want true, have it\n%v\n", rules)
	}
}

func TestName1(t *testing.T) {
	rules := []ClientRule{
		{
			ID:      "rule1",
			Address: "10.1.1.0/25",
			Length:  0,
			Name: []string{
				"claire",
				"joe",
				"nostril",
			},
		},
		{
			ID:      "rule2",
			Address: "100.1.2.0/22",
			Length:  0,
		},
		{
			ID:      "rule3",
			Address: "24.6.0.0/16",
			Length:  0,
		},
	}
	_, rules[0].Network, _ = net.ParseCIDR(rules[0].Address)
	_, rules[1].Network, _ = net.ParseCIDR(rules[1].Address)
	_, rules[2].Network, _ = net.ParseCIDR(rules[2].Address)

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

func TestRulesSimple1(t *testing.T) {
	rules := []ClientRule{
		{
			ID:      "rule1",
			Address: "10.1.1.0/25",
			Length:  0,
		},
		{
			ID:      "rule2",
			Address: "10.1.2.0/22",
			Length:  0,
		},
		{
			ID:      "rule3",
			Address: "0.0.0.0/0",
			Length:  0,
		},
	}
	_, rules[0].Network, _ = net.ParseCIDR(rules[0].Address)
	_, rules[1].Network, _ = net.ParseCIDR(rules[1].Address)
	_, rules[2].Network, _ = net.ParseCIDR(rules[2].Address)

	p := Player{
		IP: "192.168.3.5",
	}
	cl := Client{}
	match, mrules := cl.CheckRulesSimple(&p, rules)
	if !match && len(mrules) != 1 {
		t.Error(mrules)
	}

	p.IP = "10.1.1.5"
	_, mrules = cl.CheckRulesSimple(&p, rules)
	if len(mrules) != 3 {
		t.Error(p.IP, mrules)
	}

	p.IP = "10.1.3.200"
	_, mrules = cl.CheckRulesSimple(&p, rules)
	if len(mrules) != 2 {
		t.Error(p.IP, mrules)
	}
}

func TestExpired1(t *testing.T) {
	rules := []ClientRule{
		{
			ID:      "rule1",
			Address: "10.1.1.0/25",
			Length:  10,
			Name: []string{
				"claire",
				"joe",
				"nostril",
			},
		},
		{
			ID:      "rule2",
			Address: "100.1.2.0/22",
			Length:  10,
		},
		{
			ID:      "rule3",
			Address: "24.6.0.0/16",
			Length:  10,
		},
	}
	_, rules[0].Network, _ = net.ParseCIDR(rules[0].Address)
	_, rules[1].Network, _ = net.ParseCIDR(rules[1].Address)
	_, rules[2].Network, _ = net.ParseCIDR(rules[2].Address)

	//q2a.ReadGlobalRules()
	p := Player{
		ClientID: 0,
		Name:     "joe",
		IP:       "10.1.1.1",
	}
	cl := Client{}
	_, mrules := cl.CheckRules(&p, rules)
	if len(mrules) != 0 {
		t.Error("0 rule should match:", len(mrules), "\n", mrules)
	}
}

func TestReal1(t *testing.T) {
	rules := []ClientRule{
		{
			ID:      "rule1",
			Address: "10.1.1.0/25",
			Length:  0,
			Exact:   false,
			Name: []string{
				"claire",
				"joe",
				"nostril",
			},
			Password: "llbean3",
		},
		{
			ID:      "rule2",
			Address: "100.1.2.0/22",
			Length:  0,
		},
		{
			ID:      "rule3",
			Address: "24.6.0.0/16",
			Length:  0,
		},
	}
	_, rules[0].Network, _ = net.ParseCIDR(rules[0].Address)
	_, rules[1].Network, _ = net.ParseCIDR(rules[1].Address)
	_, rules[2].Network, _ = net.ParseCIDR(rules[2].Address)

	p := Player{
		ClientID: 0,
		Name:     "ostr",
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

func TestUserInfo1(t *testing.T) {
	rules := []ClientRule{
		{
			ID:      "rule1",
			Address: "10.1.1.0/25",
			Length:  0,
			Exact:   false,
			Name: []string{
				"claire",
				"joe",
				"nostril",
			},
			UserInfoKey: []string{
				"skin",
				"hand",
			},
			UserinfoVal: []string{
				"female/jezebel",
				"1",
			},
			Password: "llbean3",
		},
		{
			ID:      "rule2",
			Address: "100.1.2.0/22",
			Length:  0,
		},
		{
			ID:      "rule3",
			Address: "24.6.0.0/16",
			Length:  0,
		},
	}
	_, rules[0].Network, _ = net.ParseCIDR(rules[0].Address)
	_, rules[1].Network, _ = net.ParseCIDR(rules[1].Address)
	_, rules[2].Network, _ = net.ParseCIDR(rules[2].Address)

	p := Player{
		ClientID: 0,
		Name:     "claire",
		IP:       "10.1.1.120",
		UserinfoMap: map[string]string{
			//"pw":   "llbean3",
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

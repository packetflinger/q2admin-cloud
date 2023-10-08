package main

import (
	"testing"

	pb "github.com/packetflinger/q2admind/proto"
)

func TestUserinfoMatches(t *testing.T) {
	tests := []struct {
		desc   string
		ui     *pb.UserInfo
		player Player
		want   bool
	}{
		{
			desc: "test1",
			ui: &pb.UserInfo{
				Property: "pw",
				Value:    "dingle[bB]err.+",
			},
			player: Player{
				UserinfoMap: map[string]string{
					"pw":   "dingleberry",
					"skin": "female/jezebel",
					"hand": "1",
				},
			},
			want: true,
		},
		{
			desc: "test2",
			ui: &pb.UserInfo{
				Property: "skin",
				Value:    "cyborg/ps[0-9]+",
			},
			player: Player{
				UserinfoMap: map[string]string{
					"pw":   "blah",
					"skin": "female/jezebel",
					"hand": "1",
				},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := UserinfoMatches(tc.ui, &tc.player)
			if got != tc.want {
				t.Error("UserinfoMatches() =", got, ", want", tc.want)
			}
		})
	}
}

func TestSortRules(t *testing.T) {
	tests := []struct {
		desc  string
		rules []*pb.Rule
		want  []*pb.Rule
	}{
		{
			desc: "test1",
			rules: []*pb.Rule{
				{Type: pb.RuleType_MESSAGE},
				{Type: pb.RuleType_MESSAGE},
				{Type: pb.RuleType_BAN},
				{Type: pb.RuleType_MUTE},
				{Type: pb.RuleType_BAN},
				{Type: pb.RuleType_STIFLE},
			},
			want: []*pb.Rule{
				{Type: pb.RuleType_BAN},
				{Type: pb.RuleType_BAN},
				{Type: pb.RuleType_MUTE},
				{Type: pb.RuleType_STIFLE},
				{Type: pb.RuleType_MESSAGE},
				{Type: pb.RuleType_MESSAGE},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := SortRules(tc.rules)
			for i := range got {
				if got[i].GetType() != tc.want[i].GetType() {
					t.Error("got", got, ", want", tc.want)
				}
			}
		})
	}
}

/*
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

func TestSingleRule(t *testing.T) {
	rules := genrules2()

	p := Player{
		ClientID: 0,
		Name:     "toejam",
		IP:       "1.2.3.4",
		Hostname: "4.3-2-1.hostingstuff.net",
		UserinfoMap: map[string]string{
			"pw":   "shornscrotum",
			"hand": "2",
			"skin": "female/jezebel",
		},
	}
	cl := Client{}

	match := cl.CheckRule(&p, rules[2])
	fmt.Print(match)
	if !match {
		t.Error("Rule3 didn't match but should have")
		fmt.Println(rules[2])
	}
}
*/

package server

import (
	"testing"

	"github.com/packetflinger/q2admind/client"
	pb "github.com/packetflinger/q2admind/proto"
)

func TestUserinfoMatches(t *testing.T) {
	tests := []struct {
		desc   string
		ui     *pb.UserInfo
		player client.Player
		want   bool
	}{
		{
			desc: "test1",
			ui: &pb.UserInfo{
				Property: "pw",
				Value:    "dingle[bB]err.+",
			},
			player: client.Player{
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
			player: client.Player{
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

func TestRuleExceptionMatch(t *testing.T) {
	tests := []struct {
		desc      string
		exception *pb.Exception
		player    *client.Player
		want      bool
	}{
		{
			desc: "test1_expired",
			exception: &pb.Exception{
				ExpirationTime: 12345,
			},
			player: &client.Player{},
			want:   false,
		},
		{
			desc: "test2_name",
			exception: &pb.Exception{
				Name: []string{
					"^claire$",
					"cla..e.+",
				},
			},
			player: &client.Player{
				Name: "clairesucks",
			},
			want: true,
		},
		{
			desc: "test3_name",
			exception: &pb.Exception{
				Name: []string{
					"^claire$",
					"cla..e.+",
				},
			},
			player: &client.Player{
				Name: "ts-claire",
			},
			want: false,
		},
		{
			desc: "test4_addr",
			exception: &pb.Exception{
				Address: []string{
					"192.168.1.0/24",
				},
			},
			player: &client.Player{
				IP: "192.168.1.244",
			},
			want: true,
		},
		{
			desc: "test5_addr",
			exception: &pb.Exception{
				Address: []string{
					"192.168.1.0/24",
				},
			},
			player: &client.Player{
				IP: "192.168.2.244",
			},
			want: false,
		},
		{
			desc: "test6_ui",
			exception: &pb.Exception{
				UserInfo: []*pb.UserInfo{
					{
						Property: "skin",
						Value:    "cyborg/.+",
					},
				},
			},
			player: &client.Player{
				UserinfoMap: map[string]string{
					"skin": "cyborg/ps9000",
					"hand": "2",
				},
			},
			want: true,
		},
		{
			desc: "test7_ui",
			exception: &pb.Exception{
				UserInfo: []*pb.UserInfo{
					{
						Property: "pw",
						Value:    "^twatwaffle$", // be careful with complex passwords
					},
				},
			},
			player: &client.Player{
				UserinfoMap: map[string]string{
					"pw":   "twatwaffle",
					"hand": "2",
				},
			},
			want: true,
		},
		{
			desc: "test8_ui",
			exception: &pb.Exception{
				UserInfo: []*pb.UserInfo{
					{
						Property: "hand",
						Value:    "[12]", // be careful with complex passwords
					},
				},
			},
			player: &client.Player{
				UserinfoMap: map[string]string{
					"hand": "0",
				},
			},
			want: false,
		},
		{
			desc: "test9_ui",
			exception: &pb.Exception{
				UserInfo: []*pb.UserInfo{
					{
						Property: "hand",
						Value:    "[12]", // be careful with complex passwords
					},
				},
			},
			player: &client.Player{
				UserinfoMap: map[string]string{
					"hand": "2",
				},
			},
			want: true,
		},
		{
			desc: "test10_hostname",
			exception: &pb.Exception{
				Hostname: []string{
					"google.com",
				},
			},
			player: &client.Player{
				Hostname: "ip66xyz.google.COM",
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := RuleExceptionMatch(tc.exception, tc.player)
			if got != tc.want {
				t.Error("Got:", got, "Want:", tc.want)
			}
		})
	}
}

func TestCheckRule(t *testing.T) {
	tests := []struct {
		desc   string
		rule   *pb.Rule
		player *client.Player
		want   bool
	}{
		{
			desc: "test1_expired",
			rule: &pb.Rule{
				ExpirationTime: 12345,
			},
			player: &client.Player{},
			want:   false,
		},
		{
			desc: "test2_address",
			rule: &pb.Rule{
				Address: []string{
					"192.0.2.0/24",
					"100.64.0.0/17",
				},
			},
			player: &client.Player{
				IP: "100.64.3.16",
			},
			want: true,
		},
		{
			desc: "test3_address_name",
			rule: &pb.Rule{
				Address: []string{
					"192.0.2.0/24",
					"100.64.0.0/17",
				},
				Name: []string{
					"snooder",
				},
			},
			player: &client.Player{
				IP:   "100.64.3.16",
				Name: "claire",
			},
			want: false,
		},
		{
			desc: "test4_address_name",
			rule: &pb.Rule{
				Address: []string{
					"192.0.2.0/24",
					"100.64.0.0/17",
				},
				Name: []string{
					"snooder",
					"sn00der",
				},
			},
			player: &client.Player{
				IP:   "100.64.3.16",
				Name: "snoodersmith",
			},
			want: true,
		},
		{
			desc: "test4_address_name_exception",
			rule: &pb.Rule{
				Address: []string{
					"192.0.2.0/24",
					"100.64.0.0/17",
				},
				Name: []string{
					"snooder",
					"sn00der",
				},
				Exception: []*pb.Exception{
					{
						UserInfo: []*pb.UserInfo{
							{
								Property: "pw",
								Value:    "^to3b34ns$",
							},
						},
					},
				},
			},
			player: &client.Player{
				IP:   "100.64.3.16",
				Name: "snoodersmith",
				UserinfoMap: map[string]string{
					"pw": "to3b34ns",
				},
			},
			want: false,
		},
		{
			desc: "test5_hostname",
			rule: &pb.Rule{
				Hostname: []string{
					"rh.rit.edu",
				},
			},
			player: &client.Player{
				Hostname: "192-0-2-44.cpe.rh.rit.edu",
				Name:     "snoodersmith",
				UserinfoMap: map[string]string{
					"pw": "to3b34ns",
				},
			},
			want: true,
		},
		{
			desc: "test6_hostname",
			rule: &pb.Rule{
				Hostname: []string{
					"^192.+rh.rit.edu$",
				},
			},
			player: &client.Player{
				Hostname: "192-0-2-44.cpe.rh.rit.EDU",
				Name:     "snoodersmith",
				UserinfoMap: map[string]string{
					"pw": "to3b34ns",
				},
			},
			want: true,
		},
		{
			desc: "test7_vpn",
			rule: &pb.Rule{
				Vpn: true,
			},
			player: &client.Player{
				Name: "snoodersmith",
				VPN:  true,
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := CheckRule(tc.player, tc.rule)
			if got != tc.want {
				t.Error("Got:", got, "Want:", tc.want)
			}
		})
	}
}

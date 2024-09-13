package server

import (
	"testing"
	"time"

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
		when   time.Time
		want   bool
	}{
		{
			desc: "test1_expired",
			rule: &pb.Rule{
				ExpirationTime: 12345,
			},
			player: &client.Player{},
			when:   time.Now(),
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
			when: time.Now(),
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
			when: time.Now(),
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
			when: time.Now(),
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
			when: time.Now(),
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
			when: time.Now(),
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
			when: time.Now(),
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
			when: time.Now(),
			want: true,
		},
		{
			desc: "test8_lefty_girls",
			rule: &pb.Rule{
				UserInfo: []*pb.UserInfo{
					{
						Property: "hand",
						Value:    "2",
					},
					{
						Property: "skin",
						Value:    "female/.+",
					},
				},
			},
			player: &client.Player{
				Name: "snoodersmith",
				VPN:  true,
				UserinfoMap: map[string]string{
					"hand": "2",
					"skin": "female/jezebel",
					"rate": "8000",
				},
			},
			when: time.Now(),
			want: true,
		},
		{
			desc: "test9_time_after",
			rule: &pb.Rule{
				Timespec: &pb.TimeSpec{
					After: "2:00PM",
				},
			},
			player: &client.Player{},
			// 4:30pm on 10/5/2024
			when: time.Date(2024, time.October, 5, 16, 30, 0, 0, time.UTC),
			want: true,
		},
		{
			desc: "test9_time_before",
			rule: &pb.Rule{
				Timespec: &pb.TimeSpec{
					Before: "9:30PM",
				},
			},
			player: &client.Player{},
			// 4:30pm on 10/5/2024
			when: time.Date(2024, time.October, 5, 16, 30, 0, 0, time.UTC),
			want: true,
		},
		{
			desc: "test9_time_range_yes",
			rule: &pb.Rule{
				Timespec: &pb.TimeSpec{
					After:  "8:00AM",
					Before: "10:30PM",
				},
			},
			player: &client.Player{},
			// 4:30pm on 10/5/2024
			when: time.Date(2024, time.October, 5, 16, 30, 0, 0, time.UTC),
			want: true,
		},
		{
			desc: "test9_time_range_no",
			rule: &pb.Rule{
				Timespec: &pb.TimeSpec{
					After:  "8:00AM",
					Before: "10:30PM",
				},
			},
			player: &client.Player{},
			// 11:30pm on 10/5/2024
			when: time.Date(2024, time.October, 5, 23, 30, 0, 0, time.UTC),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := CheckRule(tc.player, tc.rule, tc.when)
			if got != tc.want {
				t.Error("Got:", got, "Want:", tc.want)
			}
		})
	}
}

func TestDurationToSeconds(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "test 1",
			input: "5.5m",
			want:  300 + 30,
		},
		{
			name:  "test 2",
			input: "1.5h",
			want:  3600 + 1800,
		},
		{
			name:  "test 3",
			input: "5:00",
			want:  300,
		},
		{
			name:  "test 4",
			input: "01:05:30",
			want:  3600 + 300 + 30,
		},
		{
			name:  "test 5",
			input: "600s",
			want:  600,
		},
		{
			name:  "test 6",
			input: "3m",
			want:  60 + 60 + 60,
		},
		{
			name:  "test 7",
			input: "930",
			want:  930,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := durationToSeconds(tc.input)
			if err != nil {
				t.Error(err)
			}
			if got != tc.want {
				t.Error("got:", got, "want:", tc.want)
			}
		})
	}
}

func TestStringToTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "test 1",
			input: "2024-10-05 15:04:05",
			want:  time.Date(2024, time.October, 5, 15, 4, 5, 0, time.UTC),
		},
		{
			name:  "test 2",
			input: "23:20:00",
			want:  time.Date(0, time.January, 1, 23, 20, 0, 0, time.UTC),
		},
		{
			name:  "test 3",
			input: "4:30PM",
			want:  time.Date(0, time.January, 1, 16, 30, 0, 0, time.UTC),
		},
		{
			name:  "test 4",
			input: "2024-10-05",
			want:  time.Date(2024, time.October, 5, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stringToTime(tc.input)
			if err != nil {
				t.Error(err)
			}
			if got != tc.want {
				t.Error("got:", got, "want:", tc.want)
			}
		})
	}
}

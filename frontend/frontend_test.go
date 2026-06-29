package frontend

import (
	//"cmp"
	"reflect"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestGetPlayerFromPrint(t *testing.T) {
	tests := []struct {
		name string
		text string
		fe   Frontend
		want []string
	}{
		{
			name: "test1",
			text: "claire: blah blah blah",
			fe: Frontend{
				Players: []Player{
					{
						Name: "claire",
					},
				},
			},
			want: []string{
				"claire",
			},
		},
		{
			name: "test2",
			text: "claire: blah blah blah",
			fe: Frontend{
				Players: []Player{
					{
						Name: "claire",
					},
					{
						Name: "claire",
					},
					{
						Name: "not-claire",
					},
				},
			},
			want: []string{
				"claire",
				"claire",
			},
		},
		{
			name: "test3",
			text: "claire: dude: blah blah blah",
			fe: Frontend{
				Players: []Player{
					{
						Name: "claire",
					},
					{
						Name: "claire",
					},
					{
						Name: "not-claire",
					},
				},
			},
			want: []string{
				"claire",
				"claire",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (tc.fe).GetPlayerFromPrint(tc.text)
			if err != nil {
				t.Error(err)
			}
			if len(got) != len(tc.want) {
				t.Error("player count mismatch. Got:", len(got), "want:", len(tc.want))
			}
			var names []string
			for _, p := range got {
				names = append(names, p.Name)
			}
			sort.Strings(names)
			if !reflect.DeepEqual(names, tc.want) {
				t.Error("\ngot", names, "\nwant", tc.want)
			}
		})
	}
}

func TestResolvePlayer(t *testing.T) {
	fe := Frontend{
		MaxPlayers: 8,
		Players: []Player{
			{
				ClientID:    0,
				ConnectTime: 100,
				Name:        "snot-rocket",
			},
			{
				ClientID:    1,
				ConnectTime: 100,
				Name:        "dingleberry",
			},
			{
				ClientID:    2,
				ConnectTime: 100,
				Name:        "  claire ",
			},
			{
				ClientID:    3,
				ConnectTime: 100,
				Name:        "  claire     ",
			},
			{
				ClientID:    4,
				ConnectTime: 100,
				Name:        "2",
			},
		},
	}

	tests := []struct {
		name     string
		frontend Frontend
		input    string
		want     []*Player
		wantErr  bool
	}{
		{
			name:     "empty",
			frontend: fe,
			input:    "",
			want:     []*Player{},
			wantErr:  true,
		},
		{
			name:     "negative player id",
			frontend: fe,
			input:    "-3",
			want:     []*Player{},
			wantErr:  true,
		},
		{
			name:     "by id",
			frontend: fe,
			input:    "1",
			want: []*Player{
				{
					ClientID:    1,
					ConnectTime: 100,
					Name:        "dingleberry",
				},
			},
			wantErr: false,
		},
		{
			name:     "by name",
			frontend: fe,
			input:    "lair",
			want: []*Player{
				{
					ClientID:    2,
					ConnectTime: 100,
					Name:        "  claire ",
				},
				{
					ClientID:    3,
					ConnectTime: 100,
					Name:        "  claire     ",
				},
			},
			wantErr: false,
		},
		{
			name:     "numeric name",
			frontend: fe,
			input:    "4",
			want: []*Player{
				{
					ClientID:    4,
					ConnectTime: 100,
					Name:        "2",
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.frontend.ResolvePlayers(tc.input)
			if (err != nil) != tc.wantErr {
				t.Error(err)
			} else {
				if diff := cmp.Diff(got, tc.want, protocmp.Transform()); diff != "" {
					t.Errorf("ResolvePlayers(%q) = %v want %v\n", tc.input, got, tc.want)
				}
			}
		})
	}
}

func TestCalculateKDR(t *testing.T) {
	tests := []struct {
		name     string
		frontend *Frontend
		id       int
		want     float64
	}{
		{
			name:     "negative id",
			frontend: &Frontend{},
			id:       -1,
			want:     0.0,
		},
		{
			name: "high id",
			frontend: &Frontend{
				MaxPlayers: 8,
			},
			id:   100,
			want: 0.0,
		},
		{
			name: "negative",
			frontend: &Frontend{
				MaxPlayers: 8,
				Players: []Player{
					{
						ClientID:    0,
						ConnectTime: 1,
						Frags:       -4,
						Deaths:      10,
					},
				},
			},
			id:   0,
			want: -2.5,
		},
		{
			name: "positive",
			frontend: &Frontend{
				MaxPlayers: 8,
				Players: []Player{
					{
						ClientID:    0,
						ConnectTime: 1,
						Frags:       25,
						Deaths:      10,
					},
				},
			},
			id:   0,
			want: 2.5,
		},
		{
			name: "zero death",
			frontend: &Frontend{
				MaxPlayers: 8,
				Players: []Player{
					{
						ClientID:    0,
						ConnectTime: 1,
						Frags:       25,
						Deaths:      0,
					},
				},
			},
			id:   0,
			want: 25,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.frontend.CalculateKDR(tc.id)
			if got != tc.want {
				t.Errorf("CalculateKDR(%d): %f, want %f\n", tc.id, got, tc.want)
			}
		})
	}
}

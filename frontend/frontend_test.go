package frontend

import (
	"reflect"
	"sort"
	"testing"
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

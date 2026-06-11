package backend

import (
	"slices"
	"testing"
)

func TestCheckmark(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "true",
			input: bool(true),
			want:  "\u2713",
		},
		{
			name:  "false",
			input: bool(false),
			want:  " ",
		},
		{
			name:  "1",
			input: int(1),
			want:  "\u2713",
		},
		{
			name:  "0",
			input: int(0),
			want:  " ",
		},
		{
			name:  "yes",
			input: string("yes"),
			want:  "\u2713",
		},
		{
			name:  "no",
			input: string("no"),
			want:  " ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := checkMark(tc.input)
			if got != tc.want {
				t.Error("got:", got, "want:", tc.want)
			}
		})
	}
}

func TestParseCmdArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  CmdArgs
	}{
		{
			name:  "empty",
			input: "",
			want: CmdArgs{
				command: "",
				argc:    0,
				argv:    []string{},
				args:    "",
			},
		},
		{
			name:  "stuff",
			input: "stuff 0 say hey there",
			want: CmdArgs{
				command: "stuff",
				argc:    4,
				argv:    []string{"0", "say", "hey", "there"},
				args:    "0 say hey there",
			},
		},
		{
			name:  "quoted say",
			input: "say \"eeny meeny miny moe\"",
			want: CmdArgs{
				command: "say",
				argc:    4,
				argv:    []string{"\"eeny", "meeny", "miny", "moe\""},
				args:    "\"eeny meeny miny moe\"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCmdArgs(tc.input)
			if err != nil {
				t.Error(err)
			}
			if got.command != tc.want.command {
				t.Error("got:", got, "want:", tc.want)
			} else if got.argc != tc.want.argc {
				t.Error("got:", got, "want:", tc.want)
			} else if got.args != tc.want.args {
				t.Error("got:", got, "want:", tc.want)
			} else if !slices.Equal(got.argv, tc.want.argv) {
				t.Error("got:", got, "want:", tc.want)
			}
		})
	}
}

package backend

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/packetflinger/q2admind/frontend"
	"golang.org/x/term"

	pb "github.com/packetflinger/q2admind/proto"
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
		{
			name:  "extra whitespace",
			input: "  kick   3\tbye ",
			want: CmdArgs{
				command: "kick",
				argc:    2,
				argv:    []string{"3", "bye"},
				args:    "3 bye",
			},
		},
		{
			name:  "whitespace only",
			input: " \t ",
			want:  CmdArgs{argv: []string{}},
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

func TestCanManage(t *testing.T) {
	owner := &pb.User{Email: "owner@example.com"}
	delegate := &pb.User{Email: "delegate@example.com"}
	stranger := &pb.User{Email: "stranger@example.com"}
	saved := be.frontends
	defer func() { be.frontends = saved }()
	be.frontends = []frontend.Frontend{{
		Name:  "fe1",
		Owner: owner.Email,
		Users: map[*pb.User][]*pb.Role{delegate: {{Context: pb.Context_SSH}}},
	}}
	fe := &be.frontends[0]
	for _, tc := range []struct {
		user *pb.User
		want bool
	}{{owner, true}, {delegate, true}, {stranger, false}} {
		if got := canManage(tc.user, fe); got != tc.want {
			t.Errorf("canManage(%s) = %v, want %v", tc.user.Email, got, tc.want)
		}
	}
}

func TestPauseExpires(t *testing.T) {
	var out bytes.Buffer
	rw := struct {
		io.Reader
		io.Writer
	}{strings.NewReader(""), &out}
	st := &SSHTerminal{terminal: term.NewTerminal(rw, "> ")}
	stream := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		linkFrontendToTerminal(ctx, &frontend.Frontend{Name: "fe1"}, st, &stream)
		close(done)
	}()

	st.mu.Lock()
	st.paused = time.Now().Unix() + PauseLength
	st.mu.Unlock()
	stream <- "held"
	st.mu.Lock()
	st.paused = time.Now().Unix() - 1 // expire it
	st.mu.Unlock()
	stream <- "after"
	cancel()
	<-done

	st.mu.Lock()
	defer st.mu.Unlock()
	if st.paused != 0 || st.buffer != nil {
		t.Errorf("pause not lifted: paused=%d buffer=%v", st.paused, st.buffer)
	}
	if got := out.String(); !strings.Contains(got, "held") || strings.Index(got, "held") > strings.Index(got, "after") {
		t.Errorf("buffered message not flushed before new one, output: %q", got)
	}
}

package backend

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

// linkedFrontend is a frontend with two players and a log captured in logged,
// whose console lines are returned by next.
func linkedFrontend(t *testing.T) (fe *frontend.Frontend, logged *bytes.Buffer, next func() string) {
	t.Helper()
	logged = &bytes.Buffer{}
	fe = &frontend.Frontend{Name: "pf-dev", UUID: "u1", MaxPlayers: 4, Log: log.New(logged, "", 0)}
	fe.Players = []frontend.Player{
		{ClientID: 0, ConnectTime: 1, Name: "claire", IP: "192.0.2.1"},
		{ClientID: 1, ConnectTime: 1, Name: "bravo", IP: "192.0.2.2"},
		{},
		{},
	}
	watch := make(chan frontend.ConsoleLine, 8)
	fe.AddTerminal(&watch, 0)
	next = func() string {
		t.Helper()
		select {
		case l := <-watch:
			return l.Text
		default:
			t.Fatal("nothing was sent to the console")
			return ""
		}
	}
	return fe, logged, next
}

const claireLink = "\x1b]8;;/sv/u1/pf-dev/player/0\x1b\\"

func TestChatNameLinks(t *testing.T) {
	fe, _, _ := linkedFrontend(t)
	if got := linkChatName(fe, "claire: gg all"); got != claireLink+"claire\x1b]8;;\x1b\\: gg all" {
		t.Errorf("chat line = %q", got)
	}
	for _, line := range []string{"(claire): team chat", "nobody: hi", "claire said hi"} {
		if got := linkChatName(fe, line); got != line {
			t.Errorf("%q should be left alone, got %q", line, got)
		}
	}
	fe.Players[1].Name = "claire" // two players with the same name
	if got := linkChatName(fe, "claire: gg"); got != "claire: gg" {
		t.Errorf("an ambiguous name shouldn't be linked, got %q", got)
	}
}

func TestPlayerLinksInConsoleNotLogs(t *testing.T) {
	fe, logged, next := linkedFrontend(t)

	ParseObituary(fe, "claire was railed by bravo")
	death := next()
	if !strings.Contains(death, "/player/1\x1b\\bravo") || !strings.Contains(death, claireLink+"claire") {
		t.Errorf("death line should link both players: %q", death)
	}

	KickPlayer(fe, &fe.Players[1], "")
	kick := next()
	if !strings.Contains(kick, "/player/1\x1b\\bravo\x1b]8;;\x1b\\"+strings.Repeat(" ", 15)+" [1]") {
		t.Errorf("kick line should link and still pad the name to 20 columns: %q", kick)
	}

	if strings.Contains(logged.String(), "\x1b") {
		t.Errorf("the log file shouldn't get link codes:\n%q", logged.String())
	}
	if !strings.Contains(logged.String(), "KICK bravo"+strings.Repeat(" ", 15)+" [1]") {
		t.Errorf("the log keeps the plain padded line:\n%s", logged.String())
	}
}

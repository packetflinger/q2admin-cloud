package backend

import (
	"log"
	"strings"
	"testing"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/frontend"
)

// hi is s as the game sends it highlighted (each byte | 0x80), decoded the
// way ReadString does: one Latin-1 character per byte.
func hi(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteRune(rune(s[i] | 0x80))
	}
	return b.String()
}

func TestQ2Plain(t *testing.T) {
	if got := hi("Item"); got != "Éôåí" {
		t.Fatalf(`hi("Item") = %q, want the reported "Éôåí"`, got)
	}
	tests := map[string]string{
		"plain text":                      "plain text",
		hi("Item") + " | " + hi("Acc."):   "Item | Acc.",
		"tab\tand\nnewline":               "tab\tand\nnewline",
		"\x10bold\x11 " + hi("\x10x\x11"): "[bold] [x]",
		"\x1d\x1e\x1e\x1f":                "----",
		"esc\x1b[31m":                     "esc.[31m", // no escapes from players
		hi(" "):                           " ",
		"\x7f\x01":                        "..",
		"José ☺":                          "Josi ☺", // é is a highlighted 'i'; beyond Latin-1 is kept
	}
	for in, want := range tests {
		if got := q2Plain(in); got != want {
			t.Errorf("q2Plain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQ2Text(t *testing.T) {
	got := q2Text(hi("Item")+" | x", ColorDefault)
	if want := "\033[32mItem\033[39m | x"; got != want {
		t.Errorf("q2Text = %q, want %q", got, want)
	}
	// ends highlighted, inside cyan text: cyan comes back
	got = q2Text("to "+hi("q2dm1"), ColorBrightCyan)
	if want := "to \033[32mq2dm1\033[96m"; got != want {
		t.Errorf("q2Text = %q, want %q", got, want)
	}
	if got := q2Text("plain", ColorDefault); got != "plain" {
		t.Errorf("plain text changed: %q", got)
	}
}

func TestVisibleWidth(t *testing.T) {
	tests := map[string]int{
		"abc":                  3,
		"\033[32mItem\033[39m": 4,
		osc8("/sv/x", "name"):  4,
		osc8("/sv/x", q2Text(hi("ab"), ColorDefault)) + "é": 3,
		"\033]unterminated": 0,
	}
	for in, want := range tests {
		if got := visibleWidth(in); got != want {
			t.Errorf("visibleWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

// Game text reaches the console readable, with the highlighted parts green,
// and the log keeps what the game sent.
func TestConsoleShowsQ2Text(t *testing.T) {
	fe := connectedFrontend(t, "admin@example.com")
	fe.MaxPlayers = 2
	fe.Players = []frontend.Player{{ClientID: 0, ConnectTime: 1, Name: hi("Bob")}, {}}
	var logged strings.Builder
	fe.Log = log.New(&logged, "", 0)
	lastLine := func() string {
		ch := make(chan frontend.ConsoleLine, 1)
		lines := fe.AddTerminal(&ch, 0)
		fe.TerminalDisconnected(&ch)
		return lines[len(lines)-1].Text
	}
	send := func(level int, text string) string {
		var m message.Buffer
		m.WriteByte(level)
		m.WriteString(text)
		fe.Message = message.NewBuffer(m.Data)
		ParsePrint(fe)
		return lastLine()
	}

	// OpenTDM's end-of-match stats header, as reported
	got := send(PRINT_HIGH, "            "+hi("Item")+" | "+hi("Acc.")+" "+hi("Kills")+"\n")
	if !strings.Contains(got, "\033[32mItem\033[30m | \033[32mAcc.\033[30m \033[32mKills") {
		t.Errorf("stats header = %q", got)
	}
	if !strings.Contains(logged.String(), hi("Item")) {
		t.Error("the log should keep the original text")
	}
	// chat from a highlighted name links the name and shows it readable
	got = send(PRINT_CHAT, hi("Bob")+": gg\n")
	if !strings.Contains(got, "/player/0\x1b\\\033[32mBob\033[32m\x1b]8;;\x1b\\: gg") {
		t.Errorf("chat line = %q", got)
	}
	// a player can't send terminal escapes to the console
	if got = send(PRINT_HIGH, "\x1b[2Jcleared?\n"); strings.Contains(got, "\x1b[2J") {
		t.Errorf("escape sequence passed through: %q", got)
	}
}

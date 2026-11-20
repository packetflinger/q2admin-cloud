package backend

import (
	"strings"
	"testing"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/frontend"
)

// signalMsg builds a CMDSignal the way q2admin's cloudSignal() does.
func signalMsg(client, event, signal int, signalName string, points, score, threshold int, name, ip string) []byte {
	var m message.Buffer
	m.WriteByte(CMDSignal)
	m.WriteByte(client)
	m.WriteByte(event)
	m.WriteLong(signal)
	m.WriteString(signalName)
	m.WriteLong(points)
	m.WriteLong(score)
	m.WriteLong(threshold)
	m.WriteString(name)
	m.WriteString(ip)
	return m.Data
}

// signalFrontend is a connected frontend with four empty slots, its
// database the test's, and its console lines captured.
func signalFrontend(t *testing.T) (*frontend.Frontend, func() []string) {
	t.Helper()
	fe, _, _ := groupTestSetup(t, "admin@example.com")
	fe.Data = &db
	fe.MaxPlayers = 4
	fe.Players = make([]frontend.Player, 4)
	ch := make(chan frontend.ConsoleLine, 100)
	fe.AddTerminal(&ch, 0)
	t.Cleanup(func() { fe.TerminalDisconnected(&ch) })
	lines := func() []string {
		var out []string
		for {
			select {
			case l := <-ch:
				out = append(out, l.Text)
			default:
				return out
			}
		}
	}
	return fe, lines
}

func parse(fe *frontend.Frontend, data ...[]byte) {
	fe.Message = message.NewBuffer(bytesJoin(data))
	ParseMessage(fe)
}

func bytesJoin(parts [][]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestParseSignal(t *testing.T) {
	fe, lines := signalFrontend(t)
	fe.Players[1] = frontend.Player{ClientID: 1, ConnectTime: 1, Name: "alpha", IP: "192.0.2.7"}

	parse(fe,
		signalMsg(1, SignalRaised, 2, "chatflood", 15, 15, 50, "alpha", "192.0.2.7"),
		signalMsg(1, SignalRaised, 1<<23, "vpn-detected", 40, 55, 50, "alpha", "192.0.2.7"),
	)
	p := &fe.Players[1]
	if p.SignalScore != 55 || p.Signals["chatflood"] != 15 || p.Signals["vpn-detected"] != 40 || fe.SignalLimit != 50 {
		t.Errorf("after two raises: score %d, signals %v, limit %d", p.SignalScore, p.Signals, fe.SignalLimit)
	}
	if got := signalList(p); got != "vpn-detected +40, chatflood +15" {
		t.Errorf("signalList = %q", got)
	}
	parse(fe, signalMsg(1, SignalCleared, 2, "chatflood", 0, 40, 50, "alpha", "192.0.2.7"))
	if _, ok := p.Signals["chatflood"]; ok || p.SignalScore != 40 {
		t.Errorf("after clearing chatflood: score %d, signals %v", p.SignalScore, p.Signals)
	}
	parse(fe, signalMsg(1, SignalRemoved, 0, "vpn-detected(40), chatflood(15)", 0, 55, 50, "alpha", "192.0.2.7"))

	out := strings.Join(lines(), "\n")
	for _, want := range []string{"chatflood +15 (score 15/50)", "SIGNAL_CLEARED:", "SIGNAL_REMOVED:", "(score 55/50): vpn-detected(40), chatflood(15)"} {
		if !strings.Contains(out, want) {
			t.Errorf("console is missing %q:\n%s", want, out)
		}
	}

	evs, err := loadSignals([]*frontend.Frontend{fe}, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 4 || evs[0].Event != SignalRemoved || evs[3].SignalName != "chatflood" || evs[3].Points != 15 ||
		evs[3].IP != "192.0.2.7" || evs[3].Server != "fe1" || evs[1].Signal != 2 {
		t.Errorf("saved signals = %+v", evs)
	}
	if evs, _ := loadSignals([]*frontend.Frontend{fe}, "bravo", 10); len(evs) != 0 {
		t.Errorf("bravo has signals %+v", evs)
	}
}

// q2admin can raise signals (a VPN, say) before it tells us about the player.
func TestSignalsBeforeConnect(t *testing.T) {
	fe, _ := signalFrontend(t)
	parse(fe, signalMsg(2, SignalRaised, 1<<23, "vpn-detected", 40, 40, 50, "alpha", "192.0.2.7"))
	var m message.Buffer
	m.WriteByte(CMDConnect)
	m.WriteByte(2)
	m.WriteString(`\name\alpha\ip\192.0.2.7:27901`)
	m.WriteString("q2pro")
	parse(fe, m.Data)
	if p := fe.Players[2]; p.ConnectTime == 0 || p.SignalScore != 40 || p.Signals["vpn-detected"] != 40 {
		t.Errorf("after connecting: %+v", p)
	}

	// and a slot whose player never connected doesn't pass them on
	parse(fe, signalMsg(3, SignalRaised, 2, "chatflood", 15, 15, 50, "bravo", "192.0.2.8"), []byte{CMDDisconnect, 3})
	if p := fe.Players[3]; p.Signals != nil || p.SignalScore != 0 {
		t.Errorf("an empty slot kept its signals after a disconnect: %+v", p)
	}
}

// Reading an unknown command's data as more commands would misread them.
func TestUnknownCommandStopsParsing(t *testing.T) {
	fe, _ := signalFrontend(t)
	fe.CurrentMap = "q2dm1"
	var m message.Buffer
	m.WriteByte(CMDMap)
	m.WriteString("q2dm2")
	parse(fe, []byte{99, CMDMap}, m.Data)
	if fe.CurrentMap != "q2dm1" {
		t.Errorf("map = %q; the frame should have been dropped after the unknown command", fe.CurrentMap)
	}
}

func TestSignalPages(t *testing.T) {
	data := samplePage()
	data.Frontend.SignalLimit = 50
	data.Player.Signals = map[string]int{"chatflood": 15}
	data.Player.SignalScore = 15
	data.Signals = []SignalEvent{
		{Time: 200, Event: SignalRemoved, SignalName: "chatflood(15), vpn-detected(40)", Score: 55, Threshold: 50, Name: "alpha", IP: "192.0.2.1"},
		{Time: 100, Event: SignalRaised, SignalName: "chatflood", Points: 15, Score: 15, Threshold: 50, Name: "alpha", IP: "192.0.2.1"},
	}
	out := renderPage(t, "server-view.tmpl", "server-view", data)
	for _, want := range []string{"Latest signals", `<span class="badge text-bg-danger">removed</span>`, "&#43;15", `<a href="/recent-players/alpha"`, `title="Signal score">15/50</span>`} { // html/template writes + as &#43;
		if !strings.Contains(out, want) {
			t.Errorf("server page is missing %q", want)
		}
	}
	out = renderPage(t, "player-view.tmpl", "player-view", data)
	if !strings.Contains(out, "<strong>15</strong>") || !strings.Contains(out, "chatflood &#43;15") {
		t.Error("player page should show the signal score and signals")
	}
	data.Signals = nil
	if out = renderPage(t, "server-view.tmpl", "server-view", data); !strings.Contains(out, "No signals yet.") {
		t.Error("server page without signals should say so")
	}
}

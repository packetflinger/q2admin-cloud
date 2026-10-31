package backend

import (
	"bytes"
	"fmt"
	"github.com/packetflinger/libq2/message"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/packetflinger/q2admind/frontend"
	pb "github.com/packetflinger/q2admind/proto"
)

func TestTeleportDestinationsSkipsBusyServers(t *testing.T) {
	self := &frontend.Frontend{Name: "self", Trusted: true}
	busy := &frontend.Frontend{Name: "busy", Trusted: true}
	idle := &frontend.Frontend{Name: "idle", Trusted: true}
	b := &Backend{frontends: []*frontend.Frontend{self, busy, idle}}
	self.Mu.Lock() // as the connection loop holds it
	busy.Mu.Lock() // mid-packet on another goroutine
	defer self.Mu.Unlock()
	defer busy.Mu.Unlock()

	done := make(chan *pb.TeleportReply)
	go func() {
		r, _ := b.teleportDestinations(self)
		done <- r
	}()
	select {
	case r := <-done:
		if r.GetEmptyServers() != "self, idle" {
			t.Errorf("EmptyServers = %q, want %q", r.GetEmptyServers(), "self, idle")
		}
	case <-time.After(time.Second):
		t.Fatal("teleportDestinations blocked on a locked frontend")
	}
	if !idle.Mu.TryLock() {
		t.Error("idle frontend left locked")
	}
}

func TestTeleportOutputOneServerPerLine(t *testing.T) {
	reply := &pb.TeleportReply{
		ActiveServers: []*pb.TeleportDestination{
			{Name: "pf-dev", Map: "q2dm1", Players: "claire"},
			{Name: "pf-tdm-dc", Map: "q2dm2", Players: "WallFly[BZZZ]"},
			{Name: "pf-tdm-de", Map: "koldduel1", Players: "WallFly[BZZZ], piu-beer"},
		},
		EmptyServers: "pf-tdm-nj",
	}
	var out bytes.Buffer
	if err := teleTmpl.Execute(&out, reply); err != nil {
		t.Fatal(err)
	}
	want := `  pf-dev                q2dm1        claire
  pf-tdm-dc             q2dm2        WallFly[BZZZ]
  pf-tdm-de             koldduel1    WallFly[BZZZ], piu-beer

Empty servers:
  pf-tdm-nj`
	if !strings.Contains(out.String(), want) {
		t.Errorf("teleport output:\n%s", out.String())
	}
}

func TestTeleportWrapsLongPlayerLists(t *testing.T) {
	var names []string
	for i := range 20 {
		names = append(names, fmt.Sprintf("player%02d", i))
	}
	reply := &pb.TeleportReply{
		ActiveServers: []*pb.TeleportDestination{{Name: "pf-dev", Map: "q2dm1", Players: strings.Join(names, ", ")}},
		EmptyServers:  strings.Repeat("an-empty-server, ", 8) + "last",
	}
	var out bytes.Buffer
	if err := teleTmpl.Execute(&out, reply); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, line := range strings.Split(text, "\n") {
		if len(line) > TeleportWidth {
			t.Errorf("line is %d characters, over %d:\n%s", len(line), TeleportWidth, line)
		}
	}
	for _, name := range names {
		if !strings.Contains(text, name) {
			t.Errorf("%s is missing from the output", name)
		}
	}
	// continuation lines line up under the players column
	if !strings.Contains(text, ",\n"+strings.Repeat(" ", teleportPlayersColumn)+"player") {
		t.Errorf("wrapped players aren't indented to the column:\n%s", text)
	}
}

func TestChunkLines(t *testing.T) {
	text := strings.Repeat("0123456789\n", 30) // 330 bytes in 11-byte lines
	chunks := chunkLines(text, 100)
	for _, c := range chunks {
		if len(c) > 100 || !strings.HasSuffix(c, "\n") {
			t.Errorf("chunk of %d bytes, or split mid-line: %q", len(c), c)
		}
	}
	if strings.Join(chunks, "") != text {
		t.Error("chunks don't add back up to the text")
	}
	long := strings.Repeat("x", 250)
	if got := chunkLines(long, 100); len(got) != 3 || strings.Join(got, "") != long {
		t.Errorf("an over-long line should be cut, got %d chunks", len(got))
	}
	if got := chunkLines("", 100); len(got) != 0 {
		t.Errorf("empty text gave %q", got)
	}
}

func TestTeleportTellsBothConsoles(t *testing.T) {
	from := &frontend.Frontend{Name: "pf-dev", UUID: "from-uuid", AllowTeleport: true, Server: &be, Log: log.New(io.Discard, "", 0)}
	from.Players = []frontend.Player{{}, {}, {}, {ClientID: 3, ConnectTime: 1, Name: "claire"}}
	to := &frontend.Frontend{Name: "pf-tdm-dc", UUID: "to-uuid", IPAddress: "100.64.1.2", Port: 27910}
	saved := be.frontends
	be.frontends = []*frontend.Frontend{from, to}
	defer func() { be.frontends = saved }()
	fromWatch, toWatch := make(chan string, 4), make(chan string, 4)
	from.AddTerminal(&fromWatch)
	to.AddTerminal(&toWatch)

	from.Message = message.NewBuffer(append([]byte{3}, append([]byte("pf-tdm-dc"), 0)...))
	Teleport(from)

	for _, tc := range []struct {
		watch chan string
		want  string
	}{
		// each server name links to that server's web console
		{fromWatch, "claire [3] teleported to \x1b]8;;/sv/to-uuid/pf-tdm-dc/console\x1b\\pf-tdm-dc\x1b]8;;\x1b\\"},
		{toWatch, "claire teleported from \x1b]8;;/sv/from-uuid/pf-dev/console\x1b\\pf-dev\x1b]8;;\x1b\\"},
	} {
		select {
		case got := <-tc.watch:
			if !strings.Contains(got, tc.want) || !strings.Contains(got, fmt.Sprint(ColorBrightCyan)) {
				t.Errorf("console got %q, want %q in cyan", got, tc.want)
			}
		default:
			t.Errorf("console never got %q", tc.want)
		}
	}
}

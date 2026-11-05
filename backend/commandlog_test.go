package backend

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/frontend"
)

// sendCommand runs a player command message through cmd as if fe had sent it:
// the player's slot, then the text they typed after the command.
func sendCommand(fe *frontend.Frontend, cmd func(*frontend.Frontend), client int, text string) {
	var m message.Buffer
	m.WriteByte(client)
	m.WriteString(text)
	fe.Message = message.NewBuffer(m.Data)
	cmd(fe)
}

func TestCommandHistory(t *testing.T) {
	up, down, _ := groupTestSetup(t, "admin@example.com") // fresh database
	down.Trusted = true
	up.AllowTeleport, up.AllowInvite, up.Enabled = true, true, true
	up.Invites = frontend.InviteBucket{Tokens: 3, Max: 3}
	up.Players = []frontend.Player{{ClientID: 0, ConnectTime: 1, Name: "alpha", IP: "192.0.2.1", InvitesAvailable: 3}}

	sendCommand(up, Teleport, 0, "")        // list the servers
	sendCommand(up, Teleport, 0, "FE2")     // matched case-insensitively
	sendCommand(up, Teleport, 0, "nowhere") // no such server
	sendCommand(up, Invite, 0, "come play rail")

	rows, err := db.Handle.Query("SELECT frontend_id, name, ip, args, destination FROM teleport ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var id int
		var name, ip, args, dest string
		rows.Scan(&id, &name, &ip, &args, &dest)
		got = append(got, fmt.Sprintf("%d %s %s %q %q", id, name, ip, args, dest))
	}
	rows.Close()
	want := []string{
		`1 alpha 192.0.2.1 "" ""`,
		`1 alpha 192.0.2.1 "FE2" "fe2"`,
		`1 alpha 192.0.2.1 "nowhere" ""`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("teleport rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var name, args string
	if err := db.Handle.QueryRow("SELECT name, args FROM invite WHERE frontend_id = 1").Scan(&name, &args); err != nil || name != "alpha" || args != "come play rail" {
		t.Errorf("invite row = %q %q, %v", name, args, err)
	}

	// an old use and another server's are left out of the span
	now := time.Now().Unix()
	db.Handle.Exec("INSERT INTO teleport (time, frontend_id, name, ip, args, destination) VALUES (?, 1, 'old', '', 'fe2', 'fe2'), (?, 2, 'other', '', 'fe1', 'fe1')", now-3*86400, now)
	st := &Stats{Times: make([]int64, statBuckets)} // 24 hours, as loadStats buckets it
	width := int64(86400 / statBuckets)
	for i := range st.Times {
		st.Times[i] = now - 86400 + int64(i)*width
	}
	cs, err := loadCommandStats(st, up)
	if err != nil {
		t.Fatal(err)
	}
	if cs.TeleportUses != 3 || cs.Teleported != 1 || cs.InviteCount != 1 {
		t.Errorf("uses %d, sent %d, invites %d; want 3, 1, 1", cs.TeleportUses, cs.Teleported, cs.InviteCount)
	}
	if got := fmt.Sprint(cs.Destinations, cs.Inviters); got != "[{fe2 1}] [{alpha 1}]" {
		t.Errorf("destinations, inviters = %s", got)
	}
	if cs.Teleports[statBuckets-1] != 3 || cs.Invites[statBuckets-1] != 1 {
		t.Errorf("latest bucket has %d teleports, %d invites; want 3, 1", cs.Teleports[statBuckets-1], cs.Invites[statBuckets-1])
	}
}

func TestServerViewCommandCharts(t *testing.T) {
	data := samplePage()
	out := renderPage(t, "server-view.tmpl", "server-view", data)
	if strings.Contains(out, "q2a-teleports-chart") {
		t.Error("charts shown without command stats")
	}
	data.Stats.Commands = &CommandStats{
		Teleports: make([]int, len(data.Stats.Times)), Invites: make([]int, len(data.Stats.Times)),
		TeleportUses: 4, Teleported: 3, Destinations: []Slice{{"pf-ffa", 2}, {"pf-tdm", 1}},
	}
	out = renderPage(t, "server-view.tmpl", "server-view", data)
	for _, want := range []string{"q2a-teleports-chart", "q2a-destinations-chart", "4 used, 3 sent", "pf-ffa", "--q2a-bars: 2", "No invites in this span."} {
		if !strings.Contains(out, want) {
			t.Errorf("server page is missing %q", want)
		}
	}
}

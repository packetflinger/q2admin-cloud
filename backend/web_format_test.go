package backend

import (
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

func TestSummarize(t *testing.T) {
	data := samplePage()
	offline := &frontend.Frontend{Name: "down", Players: []frontend.Player{{ConnectTime: 5}}}
	got := summarize(append(data.Frontends, offline))
	want := FrontendSummary{Servers: 2, Online: 1, Players: 1, Rules: 1}
	if got != want {
		t.Errorf("summarize() = %+v, want %+v", got, want)
	}
}

func TestSoftwareVersions(t *testing.T) {
	fe := &frontend.Frontend{Version: 1306, ServerVars: map[string]string{
		"version":  "q2proded r3732~7b63f040 Sep 14 2026 Linux x86_64",
		"q2admin":  "r1306~c96d8e1",
		"gamename": "baseq2",
		"gamedate": "Nov 20 2023",
	}}
	want := SoftwareVersions{
		Engine:   "q2proded r3732~7b63f040 Sep 14 2026 Linux x86_64",
		Q2Admin:  "r1306~c96d8e1",
		Game:     "baseq2",
		GameDate: "Nov 20 2023",
	}
	if got := softwareVersions(fe); got != want {
		t.Errorf("softwareVersions() = %+v, want %+v", got, want)
	}

	// no serverinfo yet: q2admin's comes from the greeting, the rest unknown
	if got := softwareVersions(&frontend.Frontend{Version: 1306}); got != (SoftwareVersions{Q2Admin: "r1306"}) {
		t.Errorf("without serverinfo got %+v", got)
	}
	// OpenTDM's revision is kept only when it's a number
	for rev, want := range map[string]string{"158": "158", "": "", "v1.2": "", "-3": ""} {
		fe := &frontend.Frontend{ServerVars: map[string]string{"gamename": "opentdm", "revision": rev}}
		if got := softwareVersions(fe).GameRev; got != want {
			t.Errorf("revision %q: GameRev = %q, want %q", rev, got, want)
		}
	}
	if got := softwareVersions(&frontend.Frontend{}); got != (SoftwareVersions{}) {
		t.Errorf("never connected got %+v", got)
	}
}

func TestSplitIPAddr(t *testing.T) {
	tests := map[string]IPAddr{
		"192.0.2.1":           {IP: "192.0.2.1"},
		"192.0.2.1:27901":     {IP: "192.0.2.1", After: ":27901"},
		"2001:db8::1":         {IP: "2001:db8::1"},
		"[2001:db8::1]:27901": {Before: "[", IP: "2001:db8::1", After: "]:27901"},
		"loopback":            {After: "loopback"},
		"":                    {},
	}
	for in, want := range tests {
		if got := splitIPAddr(in); got != want {
			t.Errorf("splitIPAddr(%q) = %+v, want %+v", in, got, want)
		}
	}
	if got := ipLensURL("2001:db8::1"); got != "https://lens.ipverse.net/2001:db8::1" {
		t.Errorf("ipLensURL = %q", got)
	}
}

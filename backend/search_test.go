package backend

import (
	"context"
	"encoding/base64"
	"slices"
	"testing"
)

func TestSearchPlayersGroupsByNameAndIP(t *testing.T) {
	groupTestSetup(t, "admin@example.com") // fresh database, which has the player table
	if _, err := db.Handle.Exec(`CREATE TABLE frontend (id INTEGER, uuid TEXT, name TEXT);
		INSERT INTO frontend VALUES (1, 'u1', 'fe1'), (2, 'u2', 'fe2'), (3, 'u3', 'theirs')`); err != nil {
		t.Fatal(err)
	}
	visit := func(server, name, ip, version, host string, vpn bool, when int64) {
		_, err := db.Handle.Exec(`INSERT INTO player (server, name, ip, hostname, vpn, cookie, version, userinfo, time)
			VALUES (?, ?, ?, ?, ?, '', ?, ?, ?)`, server, name, ip, host, vpn, version, `\name\`+name+`\ip\`+ip, when)
		if err != nil {
			t.Fatal(err)
		}
	}
	visit("fe1", "alpha", "192.0.2.1", "q2pro, built Jan 1", "a.example.net", false, 100)
	visit("fe1", "alpha", "192.0.2.1", "r1q2", "a.example.net", true, 300)
	visit("fe2", "alpha", "192.0.2.1", "q2pro, built Jan 1", "a.example.net", false, 200)
	visit("fe1", "alpha", "198.51.100.7", "q2pro, built Jan 1", "", false, 50)
	visit("theirs", "alpha", "192.0.2.1", "egl", "", false, 999) // not the searcher's server

	got, err := SearchPlayers(context.Background(), "alph", []string{"u1", "u2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d players, want 2: %+v", len(got), got)
	}
	a := got[0]
	if a.IP != "192.0.2.1" || a.Visits != 3 || a.ConnectTime != 300 || !a.VPN {
		t.Errorf("first result = %+v", a)
	}
	if !slices.Equal(a.Versions, []string{"r1q2", "q2pro, built Jan 1"}) {
		t.Errorf("versions = %q, want newest first and commas intact", a.Versions)
	}
	if !slices.Equal(a.Servers, []string{"u1", "u2"}) || !slices.Equal(a.Hostnames, []string{"a.example.net"}) {
		t.Errorf("servers = %v, hostnames = %v", a.Servers, a.Hostnames)
	}
	if a.Lookup != base64.StdEncoding.EncodeToString([]byte(`alpha\300`)) {
		t.Errorf("the result should link to the latest visit, got %q", a.Lookup)
	}
	if got[1].IP != "198.51.100.7" || got[1].Visits != 1 || len(got[1].Hostnames) != 0 {
		t.Errorf("second result = %+v", got[1])
	}
	if none, _ := SearchPlayers(context.Background(), "alph", nil); none != nil {
		t.Error("searching no servers should find nothing")
	}
}

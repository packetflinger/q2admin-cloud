package backend

import (
	"github.com/packetflinger/libq2/message"
	"github.com/packetflinger/q2admind/crypto"
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

func TestPlayerUpdateIgnoresProxyCheckName(t *testing.T) {
	userinfo := func(name string) string { return `\name\` + name + `\skin\male/grunt\cl_cookie\abc` }
	fe := &frontend.Frontend{MaxPlayers: 1}
	fe.Players = []frontend.Player{{ClientID: 0, ConnectTime: 1, Name: "alpha", UserInfoHash: crypto.MD5Hash(userinfo("alpha"))}}
	update := func(name string) {
		fe.Message = message.NewBuffer(append([]byte{0}, append([]byte(userinfo(name)), 0)...))
		ParsePlayerUpdate(fe)
	}

	update(ProxyCheckName)
	if got := fe.Players[0].Name; got != "alpha" {
		t.Errorf("after the proxy check rename, name = %q, want alpha", got)
	}
	update("alpha") // the revert, if q2admin sends it
	if got := fe.Players[0].Name; got != "alpha" {
		t.Errorf("after the revert, name = %q, want alpha", got)
	}
	update("bravo") // a real rename still works
	if got := fe.Players[0].Name; got != "bravo" {
		t.Errorf("after a real rename, name = %q, want bravo", got)
	}
}

// Only the userinfo a player connects with has their address, so a later
// update without it mustn't drop it from the userinfo box.
func TestPlayerUpdateKeepsIP(t *testing.T) {
	fe := &frontend.Frontend{MaxPlayers: 1}
	fe.Players = []frontend.Player{{ClientID: 0, ConnectTime: 1, Name: "alpha", IP: "192.0.2.1", Port: 27901,
		UserinfoMap: frontend.UserinfoMap(`\name\alpha\ip\192.0.2.1:27901`)}}
	fe.Message = message.NewBuffer(append([]byte{0}, append([]byte(`\name\bravo\skin\male/grunt`), 0, 0)...))
	ParsePlayerUpdate(fe)
	p := fe.Players[0]
	if p.Name != "bravo" || p.UserinfoMap["ip"] != "192.0.2.1" || p.UserinfoMap["port"] != "27901" || p.IP != "192.0.2.1" {
		t.Errorf("after an update without ip: name %q, userinfo ip %q port %q, IP %q", p.Name, p.UserinfoMap["ip"], p.UserinfoMap["port"], p.IP)
	}
}

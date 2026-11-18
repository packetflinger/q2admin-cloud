package backend

import (
	"github.com/packetflinger/q2admind/crypto"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

func TestValidateNewServer(t *testing.T) {
	tests := []struct {
		name, addr, port string
		wantName         string
		wantPort         int
		wantErr          bool
	}{
		{"My Server", "100.64.1.2", "", "my-server", 27910, false},
		{"pf-1", "q2.example.com", "27911", "pf-1", 27911, false},
		{"v6", "2001:db8::1", "", "v6", 27910, false},
		{"../../etc", "1.2.3.4", "", "", 0, true},
		{"", "1.2.3.4", "", "", 0, true},
		{"ok", "", "", "", 0, true},
		{"ok", "host:27910", "", "", 0, true},
		{"ok", "1.2.3.4", "70000", "", 0, true},
		{"ok", "1.2.3.4", "abc", "", 0, true},
	}
	for _, tc := range tests {
		name, port, err := validateNewServer(tc.name, tc.addr, tc.port)
		if (err != nil) != tc.wantErr || name != tc.wantName || port != tc.wantPort {
			t.Errorf("validateNewServer(%q, %q, %q) = %q, %d, %v", tc.name, tc.addr, tc.port, name, port, err)
		}
	}
}

func TestAddServerFormShowsError(t *testing.T) {
	data := samplePage()
	data.FormError = "port must be a number from 1 to 65535"
	data.Form = url.Values{"NewServerName": {"keep-me"}}
	out := renderPage(t, "server-add.tmpl", "server-add", data)
	if !strings.Contains(out, data.FormError) || !strings.Contains(out, `value="keep-me"`) {
		t.Error("form error or submitted value missing from re-rendered form")
	}
}

func TestMapNameRE(t *testing.T) {
	for _, ok := range []string{"q2dm1", "tltf", "dm/q2rdm2", "my_map-2"} {
		if !mapNameRE.MatchString(ok) {
			t.Errorf("%q should be allowed", ok)
		}
	}
	// these would inject extra console commands or are empty
	for _, bad := range []string{"", "q2dm1;quit", "q2dm1\nquit", "q2dm1 quit", `"q2dm1"`} {
		if mapNameRE.MatchString(bad) {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestServerViewStates(t *testing.T) {
	data := samplePage()
	out := renderPage(t, "server-view.tmpl", "server-view", data)
	for _, want := range []string{"Change map", ">alpha</a>", "192.0.2.1", "test ban"} {
		if !strings.Contains(out, want) {
			t.Errorf("online server page is missing %q", want)
		}
	}

	// the status box's versions and invite counts
	data.Frontend.ServerVars["version"] = "q2proded r3732~7b63f040"
	data.Frontend.ServerVars["q2admin"] = "r1306~c96d8e1"
	data.Frontend.ServerVars["gamename"] = "opentdm"
	data.Frontend.ServerVars["gamedate"] = "Nov 20 2023"
	data.Frontend.ServerVars["revision"] = "158"
	data.Frontend.Invites = frontend.InviteBucket{Tokens: 2, Max: 3, UseCount: 7}
	out = renderPage(t, "server-view.tmpl", "server-view", data)
	for _, want := range []string{
		"q2proded r3732~7b63f040", "r1306~c96d8e1", "opentdm", "(Nov 20 2023)", "r158",
		"Total Teleports", "<dt>Total Invites</dt><dd>7</dd>", "<dt>Invite tokens</dt><dd>2 / 3</dd>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status box is missing %q", want)
		}
	}

	data.Frontend.Connected = false
	data.FormError = "port must be a number from 1 to 65535"
	data.Notice = notices["saved"]
	out = renderPage(t, "server-view.tmpl", "server-view", data)
	for _, want := range []string{"isn't connected to CloudAdmin", data.FormError, notices["saved"].Text} {
		if !strings.Contains(out, want) {
			t.Errorf("offline server page is missing %q", want)
		}
	}
	if strings.Contains(out, "Change map") {
		t.Error("offline server page shouldn't offer a map change")
	}
}

func TestKeysPage(t *testing.T) {
	data := samplePage()
	data.KeyPair = &KeyPair{Private: "-----BEGIN PRIVATE KEY-----", Public: "-----BEGIN PUBLIC KEY-----"}
	out := renderPage(t, "generate-keys.tmpl", "keygen", data)
	if !strings.Contains(out, "Copy the private key now") || !strings.Contains(out, "BEGIN PRIVATE KEY") {
		t.Error("generated key pair isn't shown")
	}
	data.KeyPair = nil
	if out := renderPage(t, "generate-keys.tmpl", "keygen", data); strings.Contains(out, "Copy the private key now") {
		t.Error("key warning shown with no key pair")
	}
}

func TestPublicKeyDownload(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPair(2048)
	if err != nil {
		t.Fatal(err)
	}
	key, err := crypto.LoadPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	saved := be.publicKey
	be.publicKey = &key.PublicKey
	defer func() { be.publicKey = saved }()
	srv := httptest.NewServer(LoadWebsiteRoutes())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/public.pem") // no sign-in needed
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Disposition"), "public.pem") {
		t.Fatalf("status %d, Content-Disposition %q", res.StatusCode, res.Header.Get("Content-Disposition"))
	}
	got, err := crypto.LoadPublicKey(body) // what q2admin will load
	if err != nil {
		t.Fatalf("served key doesn't load: %v\n%s", err, body)
	}
	if !got.Equal(&key.PublicKey) {
		t.Error("served key isn't the backend's public key")
	}
}

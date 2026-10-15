package backend

import (
	"path/filepath"
	"testing"

	"github.com/packetflinger/q2admind/database"
)

func TestConfigAuthFileAndSecret(t *testing.T) {
	file := filepath.Join(t.TempDir(), "q2a.sqlite")
	d, err := database.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	// the config table as it was before auth_file and api_secret
	_, err = d.Handle.Exec(`CREATE TABLE config (debug INTEGER, verbose_level INTEGER, address TEXT, port INTEGER,
		private_key TEXT, web_enable INTEGER, web_address TEXT, web_port INTEGER, web_root TEXT, maintenance_time INTEGER,
		rpc_address TEXT, rpc_port INTEGER, log_file TEXT, ssh_address TEXT, ssh_port INTEGER, ssh_hostkey TEXT);
		INSERT INTO config VALUES (0, 0, '', 9988, '', 1, '', 8087, 'api/website', 60, '', 9989, '', '', 9922, '')`)
	if err != nil {
		t.Fatal(err)
	}
	d.Handle.Close()

	savedDB := db
	defer func() { db = savedDB }()
	if db, err = database.Open(file); err != nil { // adds the columns
		t.Fatal(err)
	}
	defer db.Handle.Close()
	c, err := be.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.GetAuthFile() != "config/oauth.pb" || c.GetApiSecret() != "" {
		t.Errorf("auth_file %q, api_secret %q", c.GetAuthFile(), c.GetApiSecret())
	}

	secret, err := apiSecret(c)
	if err != nil || len(secret) < 16 {
		t.Fatalf("generated secret %q, %v", secret, err)
	}
	c, _ = be.LoadConfig()
	if again, _ := apiSecret(c); string(again) != string(secret) {
		t.Error("generated secret wasn't saved, so sessions won't survive a restart")
	}
}

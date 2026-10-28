package database

import (
	"path/filepath"
	"testing"
)

// the rule table as it was before description and vpn were added
const oldRuleTable = `CREATE TABLE "rule" (
	"id" INTEGER, "frontend" INTEGER, "uuid" TEXT NOT NULL, "rule_type" INTEGER NOT NULL,
	"creation_time" INTEGER, "expiration_time" INTEGER DEFAULT 0, "delay" INTEGER DEFAULT 0,
	"stifle_length" INTEGER DEFAULT 60, "disabled" INTEGER DEFAULT 1, "scope" TEXT DEFAULT 'local',
	"timespec_before" TEXT, "timespec_after" TEXT, "timespec_every" TEXT, "timespec_playtime" TEXT,
	"addresses" TEXT, "names" TEXT, "hostnames" TEXT, "clients" TEXT, "userinfo" TEXT, "message" TEXT,
	PRIMARY KEY("id" AUTOINCREMENT))`

func TestOpenAddsRuleColumns(t *testing.T) {
	file := filepath.Join(t.TempDir(), "q2a.sqlite")
	d, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Handle.Exec(oldRuleTable); err != nil {
		t.Fatal(err)
	}
	d.Handle.Close()
	for range 2 { // the second open must find the columns already there
		d, err = Open(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Handle.Exec(`INSERT INTO rule (uuid, rule_type, description, vpn, server_group) VALUES ('x', 1, 'why', 1, 2)`); err != nil {
			t.Fatalf("new columns missing: %v", err)
		}
		d.Handle.Close()
	}
}

package frontend

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/packetflinger/q2admind/database"
	pb "github.com/packetflinger/q2admind/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestRuleDatabaseRoundTrip(t *testing.T) {
	d, err := database.Open(filepath.Join(t.TempDir(), "q2a.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Handle.Close()
	_, err = d.Handle.Exec(`CREATE TABLE "rule" (
		"id" INTEGER, "frontend" INTEGER, "uuid" TEXT NOT NULL, "rule_type" INTEGER NOT NULL,
		"creation_time" INTEGER, "expiration_time" INTEGER DEFAULT 0, "delay" INTEGER DEFAULT 0,
		"stifle_length" INTEGER DEFAULT 60, "disabled" INTEGER DEFAULT 1, "scope" TEXT DEFAULT 'local',
		"timespec_before" TEXT, "timespec_after" TEXT, "timespec_every" TEXT, "timespec_playtime" TEXT,
		"addresses" TEXT, "names" TEXT, "hostnames" TEXT, "clients" TEXT, "userinfo" TEXT, "message" TEXT,
		"description" TEXT, "vpn" INTEGER DEFAULT 0, "server_group" INTEGER DEFAULT 0, PRIMARY KEY("id" AUTOINCREMENT))`)
	if err != nil {
		t.Fatal(err)
	}
	fe := &Frontend{ID: 7, Data: &d}
	other := &Frontend{ID: 8, Data: &d}

	ban := &pb.Rule{
		Uuid:         "rule-1",
		Type:         pb.RuleType_BAN,
		Address:      []string{"10.0.0.0/8", "192.0.2.0/24"},
		Name:         []string{"bad.*"},
		Hostname:     []string{`\.example\.net$`},
		Client:       []string{"q2pro"},
		UserInfo:     []*pb.UserInfo{{Property: "skin", Value: "male/.*"}},
		Message:      []string{"you are banned", "appeal on discord"},
		Description:  []string{"spam bot"},
		Vpn:          true,
		StifleLength: 30,
		Timespec:     &pb.TimeSpec{After: "2026-01-01"},
	}
	if err := fe.InsertRule(ban); err != nil {
		t.Fatal(err)
	}
	if err := other.InsertRule(&pb.Rule{Uuid: "rule-2", Type: pb.RuleType_MUTE, Name: []string{"x"}}); err != nil {
		t.Fatal(err)
	}

	got, err := fe.FetchRules()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]*pb.Rule{ban}, got, protocmp.Transform()); diff != "" {
		t.Errorf("FetchRules after insert (-want +got):\n%s", diff)
	}

	ban.Description = []string{"updated"}
	ban.Disabled = true
	if err := fe.UpdateRule(ban); err != nil {
		t.Fatal(err)
	}
	got, _ = fe.FetchRules()
	if len(got) != 1 || got[0].GetDescription()[0] != "updated" || !got[0].GetDisabled() {
		t.Errorf("update not saved: %v", got)
	}

	if err := other.DeleteRule("rule-1"); err != nil { // not other's rule
		t.Fatal(err)
	}
	if got, _ := fe.FetchRules(); len(got) != 1 {
		t.Error("a frontend deleted another frontend's rule")
	}
	if err := fe.DeleteRule("rule-1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := fe.FetchRules(); len(got) != 0 {
		t.Errorf("rule not deleted: %v", got)
	}
}

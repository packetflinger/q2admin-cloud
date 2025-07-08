package frontend

import (
	"testing"
)

func MODTestSetup() *Frontend {

	players := []Player{
		{
			Name: "claire",
			IP:   "10.2.3.5",
		},
		{
			Name: "scarred",
			IP:   "192.168.4.5",
		},
	}
	fe := Frontend{
		Players: players,
	}
	return &fe
}

func TestMOD1(t *testing.T) {
	fe := MODTestSetup()
	obit := "claire feels scarred's pain"
	d, e := fe.CalculateDeath(obit)
	if e != nil {
		t.Error(e)
	}

	if d.Murderer == nil || d.Victim == nil {
		t.Error(d)
	}

	if d.Solo {
		t.Error(d)
	}

	obit = "claire saw the light"
	d, e = fe.CalculateDeath(obit)
	if e != nil {
		t.Error(e)
	}

	if d.Victim == nil {
		t.Error(d)
	}

	if d.Murderer != nil {
		t.Error(d)
	}

	if !d.Solo {
		t.Error(d)
	}
}

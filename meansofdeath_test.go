package main

import (
	"testing"
)

func MODTestSetup() *Client {

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
	cl := Client{
		Players: players,
	}
	return &cl
}

func TestMOD1(t *testing.T) {
	cl := MODTestSetup()
	obit := "claire feels scarred's pain"
	d, e := cl.CalculateDeath(obit)
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
	d, e = cl.CalculateDeath(obit)
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

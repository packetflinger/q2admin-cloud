package server

import (
	"testing"

	"github.com/packetflinger/q2admind/client"
)

func TestWriteClients(t *testing.T) {
	clients := []client.Client{
		{
			UUID:        "ljsfoiuwer",
			Name:        "test1",
			Owner:       "me@somewhere",
			Description: "just a test",
			IPAddress:   "10.1.1.1",
			Port:        27910,
			Verified:    true,
		},
		{
			UUID:        "ljsfoiuwesfsafsfr",
			Name:        "test2",
			Owner:       "me@somewhere",
			Description: "just a test",
			IPAddress:   "10.1.1.2",
			Port:        27910,
			Verified:    true,
		},
	}
	//fmt.Println(clients)
	err := WriteClients("/tmp/clients.textpb", clients)
	if err != nil {
		t.Error(err)
	}
}

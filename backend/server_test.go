package backend

import (
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

func TestWriteClients(t *testing.T) {
	frontends := []frontend.Frontend{
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
	err := MaterializeFrontends("/tmp/clients.textpb", frontends)
	if err != nil {
		t.Error(err)
	}
}

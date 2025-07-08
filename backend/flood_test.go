package backend

import "testing"

func TestTextFlood(t *testing.T) {
	TextFlood(nil, "hi")
	t.Fail()
}

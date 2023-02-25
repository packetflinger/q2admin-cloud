package main

import (
	"time"
)

// Maintenance is run concurrently to the rest
// of the program and sleeps most of the time.
// It periodically collects stats and cleans things
// up.
//
// Called from Main() in a goroutine
func (q2a *RemoteAdminServer) Maintenance() {
	for {
		time.Sleep(time.Duration(q2a.config.MaintenanceTime) * time.Second)

		// each client gets attention
		//for _, cl := range q2a.clients {
		// log # of players at this time
		//}
	}
}

package server

import (
	"time"
)

// Maintenance is run concurrently to the rest
// of the program and sleeps most of the time.
// It periodically collects stats and cleans things
// up.
//
// Called from Main() in a goroutine
func startMaintenance() {
	//s := os.PathSeparator
	for {
		time.Sleep(time.Duration(Cloud.Config.MaintenanceTime) * time.Second)

		// every so often write all the client states to disk
		/*
			if q2a.maintcount&63 == 0 {
				for _, cl := range q2a.clients {
					filename := fmt.Sprintf("client-configs%c%s.json.tests", s, cl.Name)
					cl.WriteToDisk(filename)
				}
			}
		*/
		Cloud.MaintCount++
	}
}

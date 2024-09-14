package server

import (
	"time"

	pb "github.com/packetflinger/q2admind/proto"
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
		time.Sleep(time.Duration(srv.config.MaintenanceTime) * time.Second)

		// every so often write all the client states to disk
		/*
			if q2a.maintcount&63 == 0 {
				for _, cl := range q2a.clients {
					filename := fmt.Sprintf("client-configs%c%s.json.tests", s, cl.Name)
					cl.WriteToDisk(filename)
				}
			}
		*/

		// check time-based player rules
		for _, cl := range srv.clients {
			if !cl.Connected && !cl.Trusted {
				continue
			}
			for _, p := range cl.Players {
				if p.ConnectTime == 0 {
					continue
				}

				var matches []*pb.Rule
				for _, r := range cl.Rules {
					ts := r.GetTimespec()
					if len(ts.GetPlayTime()) > 0 {
						match := CheckRule(&p, r, time.Now())
						if match {
							matches = append(matches, r)
						}
					}
					if len(ts.GetAfter()) > 0 {
						match := CheckRule(&p, r, time.Now())
						if match {
							matches = append(matches, r)
						}
					}
				}
				ApplyMatchedRules(&p, SortRules(matches))
			}
		}
		srv.maintCount++
	}
}

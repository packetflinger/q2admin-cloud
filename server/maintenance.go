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
// Called from Startup() in a goroutine
func (s *Server) startMaintenance() {
	for {
		time.Sleep(time.Duration(srv.config.MaintenanceTime) * time.Second)

		s.Logf(LogLevelDeveloperPlus, "running maintenance")
		// check time-based player rules
		for i, cl := range srv.clients {
			if !cl.Connected && !cl.Trusted {
				continue
			}
			s.clients[i].Invites.InviteBucketAdd()
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

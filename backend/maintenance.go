package backend

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
func (s *Backend) startMaintenance() {
	be.Logf(LogLevelInfo, "starting maintenance thread (%d secs)\n", s.config.GetMaintenanceTime())
	for {
		time.Sleep(time.Duration(be.config.MaintenanceTime) * time.Second)

		s.Logf(LogLevelDeveloperPlus, "running maintenance")
		now := time.Now().Unix()
		var samples []statsSample
		// check time-based player rules
		for _, cl := range be.Frontends() {
			// network call, done before taking the lock
			vars, err := cl.FetchServerVars()
			cl.Mu.Lock()
			if !cl.Connected && !cl.Trusted {
				cl.Mu.Unlock()
				continue
			}
			cl.Invites.InviteBucketAdd()
			for i := range cl.Players {
				p := &cl.Players[i]
				if p.ConnectTime == 0 {
					continue
				}

				var matches []*pb.Rule
				for _, r := range cl.Rules {
					ts := r.GetTimespec()
					if len(ts.GetPlayTime()) > 0 {
						match := CheckRule(p, r, time.Now())
						if match {
							matches = append(matches, r)
						}
					}
					if len(ts.GetAfter()) > 0 {
						match := CheckRule(p, r, time.Now())
						if match {
							matches = append(matches, r)
						}
					}
				}
				ApplyMatchedRules(p, SortRules(matches))
			}

			if err != nil {
				be.Logf(LogLevelInfo, "error fetching %q vars: %v", cl.Name, err)
			}
			cl.ServerVars = vars
			samples = append(samples, statsSample{frontendID: cl.ID, players: playersOnline(cl), kills: cl.Kills})
			cl.Kills = nil
			cl.Mu.Unlock()
		}
		if err := saveSamples(now, samples); err != nil {
			be.Logf(LogLevelNormal, "error saving stats samples: %v", err)
		}
		be.maintCount++
	}
}

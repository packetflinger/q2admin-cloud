package frontend

import (
	"fmt"
	"testing"
)

func TestConsoleBacklog(t *testing.T) {
	fe := &Frontend{}
	for i := 1; i <= ConsoleBacklog+20; i++ {
		fe.ConsoleBroadcast(fmt.Sprint("line ", i))
	}
	all := fe.AddTerminal(new(chan ConsoleLine), 0)
	if len(all) != ConsoleBacklog || all[0].Text != "line 21" || all[len(all)-1].Text != fmt.Sprint("line ", ConsoleBacklog+20) {
		t.Errorf("backlog has %d lines from %q to %q, want the newest %d in order",
			len(all), all[0].Text, all[len(all)-1].Text, ConsoleBacklog)
	}
	if all[0].Time == 0 {
		t.Error("lines should record when they happened")
	}

	last := all[len(all)-1].Seq
	if got := fe.AddTerminal(new(chan ConsoleLine), last-2); len(got) != 2 || got[0].Seq != last-1 {
		t.Errorf("resuming after %d gave %d lines, want the 2 after it", last-2, len(got))
	}
	if got := fe.AddTerminal(new(chan ConsoleLine), last); len(got) != 0 {
		t.Errorf("an up-to-date watcher got %d lines", len(got))
	}
	if got := fe.AddTerminal(new(chan ConsoleLine), last+100); len(got) != ConsoleBacklog {
		t.Errorf("an id from before a restart should get the whole backlog, got %d lines", len(got))
	}
}

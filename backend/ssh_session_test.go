package backend

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	pb "github.com/packetflinger/q2admind/proto"
)

// lockedBuffer is a bytes.Buffer safe to read while an SSH session writes it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// runSSHSession logs in to a test SSH server running sessionHandler as user,
// types each command, and returns everything the session printed. The
// session must end by itself (the last command quits).
func runSSHSession(t *testing.T, user string, commands ...string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &ssh.Server{Handler: sessionHandler} // no auth handlers: anyone may log in
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	client, err := gossh.Dial("tcp", ln.Addr().String(), &gossh.ClientConfig{
		User:            user,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	var out lockedBuffer
	sess.Stdout = &out
	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.Shell(); err != nil {
		t.Fatal(err)
	}
	for _, c := range commands {
		stdin.Write([]byte(c + "\r"))
		time.Sleep(20 * time.Millisecond) // let each command finish before the next
	}
	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("session didn't end after its commands; output:\n%s", out.String())
	}
	return out.String()
}

// Drives a whole SSH session through sessionHandler's command dispatch.
func TestSSHSessionCommands(t *testing.T) {
	owner := "admin@example.com"
	up, _, _ := groupTestSetup(t, owner)
	savedUsers := be.users
	be.users = []*pb.User{{Email: owner}}
	t.Cleanup(func() { be.users = savedUsers })

	out := runSSHSession(t, owner,
		"help",            // before picking a server: the short help
		"say too early",   // needs a server, ignored
		"server",          // lists servers
		"server nosuch",   // unknown
		"server fe1",      // start managing fe1
		"help",            // now the full help
		"say hello there", // goes to the server
		"kick 9",          // no such player
		"mute",            // lists mutes
		"rules show zzzz", // no match, prints nothing
		"bogus",           // unknown commands are ignored
		"quit",
	)
	for _, want := range []string{
		"You need to use the server command", // short help
		"fe1",                                // in the server list
		`server: unable to locate "nosuch"`,
		"rcon <cmd>", // full help
		"hello there",
		"kick: ",
		"Active mutes:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("session output is missing %q:\n%s", want, out)
		}
	}
	// the terminal echoes what's typed, so look for say's own colored line
	if strings.Contains(out, magenta("too early")) {
		t.Error("say ran without a server selected")
	}
	if !strings.Contains(out, magenta("hello there")) {
		t.Error("say didn't run once a server was selected")
	}
	if !up.Mu.TryLock() {
		t.Fatal("fe1 is still locked after the session ended")
	}
	up.Mu.Unlock()
}

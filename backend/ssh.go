// An SSH server is used to provide interactive access to Quake 2 server operators
package backend

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/packetflinger/q2admind/frontend"
	"golang.org/x/term"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
	gossh "golang.org/x/crypto/ssh"
)

const (
	TopLevelPrompt = "q2a"
	PauseLength    = 600 // 10 minutes
	// Drop SSH connections with no reads or writes for this long. Console
	// output counts as activity, so watching a busy server stays connected.
	SSHIdleTimeout = 30 * time.Minute
)

// SSHTerminal is a basic wrapper to enable making it easier to write data
// to the *term.Terminal pointer for this SSH session
type SSHTerminal struct {
	// What we're wrapping
	terminal *term.Terminal
	// Displayed to the left of the cursor while waiting for input
	prompt string
	// A unix timestamp of when the terminal pause will expire. If this value
	// is greater than 0, the terminal is "paused" and new incoming messages
	// should be buffered. We use a timestamp here to prevent a terminal from
	// being paused for an extended period of time slowly sucking up memory to
	// store the buffer.
	paused int64
	// This is where incoming messages are stored while the terminal is paused.
	// When the terminal is resumed, these messages are sent to the terminal
	// and the structure set to nil.
	buffer []string
	// Guards paused and buffer, shared with the console stream goroutine
	mu sync.Mutex
}

// Start listening for SSH connections
func (b *Backend) startSSHServer() {
	var hostkey ssh.Signer
	if keydata := be.config.GetSshHostkey(); keydata == "" {
		b.Logf(LogLevelNormal, "no SSH host key configured, generating a temporary one")
	} else {
		var err error
		hostkey, err = CreateHostKeySigner([]byte(keydata))
		if err != nil {
			// a temporary key would trigger host-key-changed warnings for every user
			log.Fatalf("SSH host key error: %v", err)
		}
	}
	sv := &ssh.Server{
		Addr: fmt.Sprintf("%s:%d",
			be.config.GetSshAddress(),
			be.config.GetSshPort(),
		),
		Handler:          sessionHandler,
		PublicKeyHandler: publicKeyHandler,
		IdleTimeout:      SSHIdleTimeout,
	}
	if hostkey != nil {
		sv.AddHostKey(hostkey) // has to be set outside server config creation
	}
	b.Logf(LogLevelNormal, "listening for SSH connections on %s", sv.Addr)
	log.Fatal(sv.ListenAndServe())
}

// CreateHostKeySigner will return a Signer struct based on a private key used
// as the host key.
//
// If you don't specify a host key to identify the server at
// startup, the server will generate a new one every time. This will result in
// those super annoying "WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!"
// errors when reconnecting to the same server.
//
// You can generate a keypair using commands like:
//
//	ssh-keygen -t rsa -b 1024  # usually go with a high bit length
//	ssh-keygen -t ecdsa -b 521
func CreateHostKeySigner(keydata []byte) (ssh.Signer, error) {
	s, err := gossh.ParsePrivateKey(keydata)
	if err != nil {
		return nil, fmt.Errorf("parsing private key data: %v", err)
	}
	return s, nil
}

// sshSession is one logged-in SSH user's state while sessionHandler runs.
type sshSession struct {
	s      ssh.Session
	term   *SSHTerminal
	active *frontend.Frontend // the server being managed, nil for none
	cancel context.CancelFunc // stops the active server's console stream
	locked *frontend.Frontend // locked while a command runs, never while waiting for input
}

// unlock releases the frontend locked for the current command, if any.
func (ss *sshSession) unlock() {
	if ss.locked != nil {
		ss.locked.Mu.Unlock()
		ss.locked = nil
	}
}

// sessionHandler is the "main" function for an SSH session. Once a user is
// logged in, this is concurrently called. If this function returns, the
// session is over and the connection is closed.
//
// This function will block while waiting for user input. A separate (internal)
// go routine is run for accepting client messages for outputting (prints,
// join/parts, etc) at the same time as waiting for user input.
//
// The user will have to select which of their servers to monitor via the
// terminal by using the "server" command. With no argument, all accessible
// servers will be listed.
func sessionHandler(s ssh.Session) {
	ss := &sshSession{
		s:      s,
		term:   &SSHTerminal{terminal: term.NewTerminal(s, "> ")},
		cancel: func() {},
	}
	ss.term.SetPrompt(TopLevelPrompt, true)

	defer be.Logf(LogLevelInfo, "SSH user %q [%s] disconnected\n", s.User(), s.RemoteAddr().String())
	defer s.Close()
	defer func() { ss.cancel() }() // closure: cancel is replaced on server switch
	defer ss.unlock()              // every command unlocks at the top of the loop

	for {
		ss.unlock()
		line, err := ss.term.terminal.ReadLine()
		if err != nil {
			return
		}
		if ss.active != nil {
			ss.active.Mu.Lock()
			ss.locked = ss.active
		}
		if ss.active != nil && !ss.active.Connected { // server dropped
			ss.term.Printf(yellow("** server connection to %s dropped **\n"), ss.active.Name)
			ss.cancel()
			ss.term.SetPrompt(TopLevelPrompt, true)
			ss.active = nil
			continue
		}
		c, err := ParseCmdArgs(line)
		if err != nil {
			ss.term.Println(err.Error())
		}

		// commands that work with or without a server selected
		switch c.command {
		case "server", "servers":
			ss.unlock() // checking access locks servers, which may include this one
			ss.cmdServer(c)
			continue // the active server may have changed and isn't locked yet
		case "group", "groups":
			ss.unlock() // group commands lock their servers, which may include this one
			ss.cmdGroup(c)
			continue
		case "quit", "exit", "logout", "q":
			return
		}
		if ss.active == nil {
			if c.command == "help" || c.command == "?" {
				ss.cmdTopHelp()
			}
			continue
		}
		ss.runServerCommand(c)
		SendMessages(ss.active)
	}
}

// runServerCommand runs a command that needs a server selected, on the
// active server. Unknown commands are ignored.
func (ss *sshSession) runServerCommand(c CmdArgs) {
	switch c.command {
	case "say":
		ss.cmdSay(c)
	case "help", "?":
		ss.cmdHelp()
	case "whois":
		ss.cmdWhois(c)
	case "stuff":
		ss.cmdStuff(c)
	case "rcon":
		ss.cmdRcon(c)
	case "status":
		ss.cmdStatus()
	case "consolesay":
		ss.cmdConsoleSay(c)
	case "sayperson":
		ss.cmdSayPerson(c)
	case "kick":
		ss.cmdKick(c)
	case "mute":
		ss.cmdMute(c)
	case "stifle", "stifled", "stifles":
		ss.cmdStifle(c)
	case "pause":
		ss.cmdPause()
	case "unpause":
		ss.term.unpause()
	case "search":
		ss.cmdSearch(c)
	case "rules", "rule":
		ss.cmdRules(c)
	case "settings":
		ss.term.Printf("%s\n", prototext.Format(ss.active.ToProto()))
	}
}

// publicKeyHandler provides key-based authentication for the internal SSH
// server. Keys are generated as users are created via the website. The user
// can login and download their private key to use for SSH access. Transfering
// the private key is not the best idea, but there really isn't a way of
// getting around that if the keys are generated on the server. The user needs
// to not be `disabled` and the `allow_ssh` property needs to be true.
//
// key argument is derived from the private key on the SSH client's side. The
// username is passed in via the context.
//
// Return true to allow the connection, false to deny.
func publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	var usr *pb.User
	for _, u := range be.users {
		if u.GetEmail() == ctx.User() {
			usr = u
		}
	}
	if usr == nil {
		return false
	}
	if usr.GetDisabled() {
		be.Logf(LogLevelInfo, "SSH login from disabled user %q disallowed\n", usr.GetEmail())
		return false
	}
	if !usr.GetAllowSsh() {
		be.Logf(LogLevelInfo, "SSH login from %q disallowed via config\n", usr.GetEmail())
		return false
	}
	pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(usr.GetPublicKey()))
	if err != nil {
		be.Logf(LogLevelAll, "SSH error parsing public key: %v\n", err)
		return false
	}
	if ssh.KeysEqual(key, pub) {
		be.Logf(LogLevelInfo, "SSH user %q [%s] allowed using key %q\n", ctx.User(), ctx.RemoteAddr().String(), comment)
		return true
	}
	return false
}

// canManage reports whether u owns fe or has had it delegated to them.
func canManage(u *pb.User, fe *frontend.Frontend) bool {
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	return fe.CanAccess(u.GetEmail())
}

// User returns a user proto for the given email address
func User(email string) (*pb.User, error) {
	if email == "" {
		return nil, fmt.Errorf("blank email input")
	}
	for i := range be.users {
		if be.users[i].Email == email {
			return be.users[i], nil
		}
	}
	return nil, fmt.Errorf("User(%q): unable to locate user", email)
}

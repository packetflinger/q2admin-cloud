// An SSH server is used to provide interactive access to Quake 2 server operators
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/util"
	"golang.org/x/term"

	pb "github.com/packetflinger/q2admind/proto"
	gossh "golang.org/x/crypto/ssh"
)

const (
	ColorReset         = 0
	ColorBlack         = 30
	ColorRed           = 31
	ColorGreen         = 32
	ColorYellow        = 33
	ColorBlue          = 34
	ColorMagenta       = 35
	ColorCyan          = 36
	ColorLightGray     = 37
	ColorDarkGray      = 90
	ColorBrightRed     = 91
	ColorBrightGreen   = 92
	ColorBrightYellow  = 93
	ColorBrightBlue    = 94
	ColorBrightMagenta = 95
	ColorBrightCyan    = 96
	ColorWhite         = 97
	AnsiReset          = "\033[m"
)
const (
	TermMsgTypeGeneral = iota
	TermMsgTypePlayerChat
	TermMsgTypeBan
	TermMsgTypeMute
)

type ansiCode struct {
	foreground int
	background int
	bold       bool
	underlined bool
	inversed   bool
}

type CmdArgs struct {
	command string
	argc    int
	argv    []string
	args    string
}

// SSHTerminal is a basic wrapper to enable making it easier to write data
// to the *term.Terminal pointer for this SSH session
type SSHTerminal struct {
	terminal *term.Terminal
}

// TerminalMessage is a struct that is sent from the client to the SSH server
// to display. This allows for coloring certain information, etc.
type TerminalMessage struct {
	Type       int
	PlayerName string
	PlayerID   int
	Message    string
}

// Start listening for SSH connections
func (s *Server) startSSHServer() {
	hostkey, err := CreateHostKeySigner(srv.config.GetSshHostkey())
	if err != nil {
		s.Logf(LogLevelNormal, "SSH host key error: %v", err)
	}
	sv := &ssh.Server{
		Addr: fmt.Sprintf("%s:%d",
			srv.config.GetSshAddress(),
			srv.config.GetSshPort(),
		),
		Handler:          sessionHandler,
		PublicKeyHandler: publicKeyHandler,
	}
	if hostkey != nil {
		sv.AddHostKey(hostkey) // has to be set outside server config creation
	}
	s.Logf(LogLevelNormal, "listening for SSH connections on %s", sv.Addr)
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
func CreateHostKeySigner(keyfile string) (ssh.Signer, error) {
	data, err := os.ReadFile(keyfile)
	if err != nil {
		return nil, fmt.Errorf("CreateHostkeySigner(%q): %v", keyfile, err)
	}
	s, err := gossh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("ParsePrivateKey() in CreateHostkeySigner(%q): %v", keyfile, err)
	}
	return s, nil
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
	var cl *client.Client
	var activeClient *client.Client
	var ctx context.Context
	var cancel context.CancelFunc
	sshterm := SSHTerminal{terminal: term.NewTerminal(s, "q2a> ")}

	for {
		line, err := sshterm.terminal.ReadLine()
		if err != nil {
			break
		}
		if activeClient != nil && !activeClient.Connected { // server dropped
			sshterm.Printf("** server connection to %s dropped **\n", activeClient.Name)
			cancel()
			closeClientTerminalChannel(cl)
			sshterm.terminal.SetPrompt("q2a> ")
			activeClient = nil
			continue
		}
		c, err := ParseCmdArgs(line)
		if err != nil {
			sshterm.Println(err.Error())
		}

		if c.command == "server" || c.command == "servers" {
			msg := ""
			if c.argc == 0 {
				msg, err = MyServersResponse(s)
				if err != nil {
					sshterm.Println(err.Error())
					continue
				}
			} else {
				cl, err = srv.FindClientByName(c.argv[0])
				if err != nil {
					sshterm.Printf("server: unable to locate %q\n", c.argv[0])
					continue
				}
				if !(cl.Connected && cl.Trusted) {
					sshterm.Printf("%q is offline, it can't be managed currently\n", c.argv[0])
					continue
				}
				if cl.TermCount > 0 {
					cl.TermCount--
				}
				if cancel != nil {
					cancel()
				}
				closeClientTerminalChannel(cl)
				activeClient = cl
				ctx, cancel = context.WithCancel(context.Background())
				go linkClientToTerminal(ctx, activeClient, sshterm)
				defer cancel()
				sshterm.terminal.SetPrompt("q2a/" + cl.Name + "> ")
			}
			sshterm.Println(msg)
		}
		if c.command == "quit" || c.command == "exit" || c.command == "logout" || c.command == "q" {
			break
		}
		if (c.command == "help" || c.command == "?") && activeClient == nil {
			msg := "Available commands:\n"
			msg += "  help               - show this message\n"
			msg += "  quit               - close the ssh connection\n"
			msg += "  server [name]      - switch management servers\n"
			msg += "                       omitting [name] will list possible servers\n"
			msg += "\nYou need to use the server command to connect to a management server"
			sshterm.Println(msg)
		}
		if activeClient == nil {
			continue
		}
		if c.command == "say" {
			if c.argc == 0 {
				sshterm.Println("Usage: say <something_to_say>")
				continue
			}
			sshterm.Println(magenta(c.args))
			SayEveryone(cl, PRINT_CHAT, c.args)
		}
		if c.command == "help" || c.command == "?" {
			msg := "Available commands:\n"
			msg += "  help               - show this message\n"
			msg += "  mute <#> <secs>    - mute player # for secs seconds\n"
			msg += "  quit               - close the ssh connection\n"
			msg += "  rcon <cmd>         - execute <cmd> on the remote server\n"
			msg += "  say <text>         - broadcasts <text> to all players\n"
			msg += "  search <string>    - search player records (names, hosts, userinfo, etc)\n"
			msg += "  server [name]      - switch management servers\n"
			msg += "                       omitting [name] will list possible servers\n"
			msg += "  stuff <#> <cmd>    - force client # to do a command\n"
			msg += "  whois <#>          - show player info for client #\n"
			sshterm.Println(msg)
		}
		if c.command == "whois" {
			if len(c.args) == 0 {
				activeClient.SSHPrintln("Usage: whois <id>")
				continue
			}
			pid, err := strconv.Atoi(c.argv[0])
			if err != nil {
				sshterm.Println("whois error: " + err.Error())
				continue
			}
			if pid < 0 || pid > activeClient.MaxPlayers {
				sshterm.Printf("whois error: invalid player ID: %d\n", pid)
				continue
			}
			p := activeClient.Players[pid]
			if p.ConnectTime == 0 {
				sshterm.Printf("whois: client_id %q not in use\n", c.argv[0])
				continue
			}
			msg := p.Dump()
			sshterm.Println(msg)
		}
		if c.command == "stuff" {
			if len(c.args) == 0 {
				sshterm.Println("Usage: stuff <id> <command>")
				continue
			}
			pid, err := strconv.Atoi(c.argv[0])
			if err != nil {
				sshterm.Println("stuff error: " + err.Error())
				continue
			}
			if pid < 0 || pid > activeClient.MaxPlayers {
				sshterm.Printf("stuff error: invalid player ID: %d\n", pid)
				continue
			}
			p := &activeClient.Players[pid]
			if p.ConnectTime == 0 {
				sshterm.Printf("stuff: client_id %q not in use\n", c.argv[0])
				continue
			}
			StuffPlayer(cl, p, strings.Join(c.argv[1:], " "))
		}
		if c.command == "rcon" {
			// this is not a real rcon command (out-of-band over UDP), just
			// simulated over existing TCP connection
			if len(c.args) == 0 {
				sshterm.Println("Usage: rcon <command>")
				continue
			}
			ConsoleCommand(activeClient, c.args)
		}
		if c.command == "status" {
			sshterm.Println(activeClient.StatusString())
		}
		if c.command == "consolesay" {
			if len(c.args) == 0 {
				sshterm.Println("Usage: consolesay <message>")
				continue
			}
			ConsoleSay(cl, c.args)
			cl.Log.Println("console:", c.args)
			sshterm.Println("console: " + c.args)
		}
		if c.command == "sayperson" {
			if len(c.args) == 0 {
				sshterm.Println("Usage: sayplayer <id> [message]")
				continue
			}
			id, err := strconv.Atoi(c.argv[0])
			if err != nil {
				sshterm.Printf("sayplayer: invalid client_id %q\n", c.argv[0])
				continue
			}
			if id < 0 || id > activeClient.MaxPlayers {
				sshterm.Printf("sayplayer: invalid client_id %q\n", c.argv[0])
				continue
			}
			p := &activeClient.Players[id]
			if p.ConnectTime == 0 {
				sshterm.Printf("sayperson: client_id %q not in use\n", c.argv[0])
				continue
			}
			SayPlayer(cl, p, PRINT_CHAT, strings.Join(c.argv[1:], " "))
		}
		if c.command == "kick" {
			if len(c.args) == 0 {
				sshterm.Println("Usage: kick <id> [message]")
				continue
			}
			id, err := strconv.Atoi(c.argv[0])
			if err != nil {
				sshterm.Printf("kick: invalid client_id %q\n", c.argv[0])
				continue
			}
			if id < 0 || id > activeClient.MaxPlayers {
				sshterm.Printf("kick: invalid client_id %q\n", c.argv[0])
				continue
			}
			p := &activeClient.Players[id]
			if p.ConnectTime == 0 {
				sshterm.Printf("kick: client_id %q not in use\n", c.argv[0])
				continue
			}
			KickPlayer(cl, p, strings.Join(c.argv[1:], " "))
		}
		if c.command == "mute" {
			if len(c.args) == 0 { // list all mutes
				sshterm.Println("Active mutes:")
				details := ""
				for _, m := range activeClient.Rules {
					if m.Type != pb.RuleType_MUTE {
						continue
					}
					mtxt, err := RuleDetailLine(m)
					if err != nil {
						sshterm.Printf(" mute list error: %v\n", err)
						continue
					}
					details += mtxt + "\n"
				}
				sshterm.Println(details + "\nUsage: mute <player_id> <seconds>")
				continue
			}
			id, err := strconv.Atoi(c.argv[0])
			if err != nil {
				sshterm.Printf("mute: invalid client_id %q\n", c.argv[0])
				continue
			}
			if id < 0 || id > activeClient.MaxPlayers {
				sshterm.Printf("mute: invalid client_id %q\n", c.argv[0])
				continue
			}
			secs, err := strconv.Atoi(c.argv[1])
			if err != nil {
				sshterm.Printf("mute: invalid seconds %q\n", c.argv[1])
				continue
			}
			p := &activeClient.Players[id]
			if p.ConnectTime == 0 {
				sshterm.Printf("mute: client_id %q not in use\n", c.argv[0])
				continue
			}
			MutePlayer(cl, p, secs)
		}
		if c.command == "pause" {
			if activeClient.TermPaused {
				continue
			}
			activeClient.TermPaused = true
			prompt := fmt.Sprintf("%s%s%s>",
				ansiCode{foreground: ColorBrightRed}.Render(),
				activeClient.Name,
				AnsiReset,
			)
			sshterm.terminal.SetPrompt(prompt)
		}
		if c.command == "unpause" {
			if !activeClient.TermPaused {
				continue
			}
			activeClient.TermPaused = false
			for _, line := range activeClient.TermBuf {
				sshterm.Println(line)
			}
			prompt := fmt.Sprintf("%s>", activeClient.Name)
			sshterm.terminal.SetPrompt(prompt)
			activeClient.TermBuf = []string{}
		}
		if c.command == "search" {
			if c.argc == 0 {
				sshterm.Println("Usage: search <partial_name_ip_host_userinfo>")
				continue
			}
			res, err := db.Search(c.args)
			if err != nil {
				sshterm.Printf("database.Search(%q): %v\n", c.args, err)
				continue
			}
			var out string
			for _, r := range res {
				out += fmt.Sprintf("%-6d %-15s %-15s %-15s %s\n", r.ID, r.Server, r.Name, r.IP, util.TimeAgo(r.Time))
			}
			if len(out) > 0 {
				out = fmt.Sprintf("------ --------------- --------------- --------------- ---------\n%s", out)
				out = fmt.Sprintf("%-6s %-15s %-15s %-15s %s%s", "ID", "Server", "Name", "IP", "Last Seen\n", out)
			}
			sshterm.Println(out)
			continue
		}
	}
}

// linkClientToTerminal connects the client to the the ssh terminal to receive and
// display console messages. This is one-way from Q2 server to terminal to
// show things like connects, disconnects, chats, and internal q2admin stuff.
//
// The ssh user can select which client to watch. This is run concurrently
// and is stopped when the user switches clients.
//
// The context arg is a "withCancel" context, so the calling func can terminate
// this go routine even when it's blocking waiting for input if needed.
func linkClientToTerminal(ctx context.Context, cl *client.Client, t SSHTerminal) {
	var now string
	var msg string
	//var logmsg string
	msg = fmt.Sprintf("* linking terminal to %s *", cl.Name)
	t.Println(yellow(msg))
	/*
		if cl.TermCount == 0 {
			cl.TermLog = make(chan string)
		}
	*/

	cl.TermCount++
	for {
		select {
		case logmsg := <-cl.TermLog:
			now = time.Now().Format("15:04:05")
			msg = fmt.Sprintf("%s %q\n", now, logmsg)
			if cl.TermPaused {
				cl.TermBuf = append(cl.TermBuf, msg)
			} else {
				t.Println(msg)
			}
		case <-ctx.Done():
			msg = fmt.Sprintf("* unlinking %s *", cl.Name)
			t.Println(msg)
			return
		}
	}
}

// closeClientTerminalChannel will close terminal receive channel if there
// no active terminal connections.
func closeClientTerminalChannel(cl *client.Client) {
	if cl.TermCount == 0 && cl.TermLog != nil {
		close(cl.TermLog)
	}
}

// publicKeyHandler provides key-based authentication for the internal SSH
// server. Keys are generated as users are created via the website. The user
// can login and download their private key to use for SSH access. Transfering
// the private key is not the best idea, but there really isn't a way of
// getting around that if the keys are generated on the server.
//
// key argument is derived from the private key on the SSH client's side. The
// username is passed in via the context.
//
// Return true to allow the connection, false to deny.
func publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	for _, u := range srv.users {
		if u.GetEmail() == ctx.User() {
			pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(u.GetPublicKey()))
			if err != nil {
				fmt.Printf("publicKeyHandler error: %v\n", err)
				return false
			}
			if ssh.KeysEqual(key, pub) {
				return true
			}
		}
	}
	return false
}

// Render will build an ANSI color code based on the receiver. This is only
// used when sending strings to an SSH terminal.
func (c ansiCode) Render() string {
	b := 22
	if c.bold {
		b = 1
	}
	u := 24
	if c.underlined {
		u = 4
	}
	r := 27
	if c.inversed {
		r = 7
	}
	return fmt.Sprintf("\033[0;%d;%d;%d;%d;%dm", c.foreground, c.background+10, b, u, r)
}

// ParseCmdArgs breaks up the current SSH command and args
func ParseCmdArgs(input string) (CmdArgs, error) {
	if len(input) == 0 {
		return CmdArgs{}, errors.New("empty input")
	}
	tokens := strings.Split(strings.Trim(input, " \n\t"), " ")
	if len(tokens) == 0 {
		return CmdArgs{}, fmt.Errorf("ParseCmdArgs(%q) - can't parse command", input)
	}
	return CmdArgs{
		command: strings.ToLower(tokens[0]),
		argc:    len(tokens) - 1,
		argv:    tokens[1:],
		args:    strings.Join(tokens[1:], " "),
	}, nil
}

// Println will send str to the SSH terminal. If the input string is missing
// a newline, it's added before sending.
func (t SSHTerminal) Println(str string) {
	if !strings.HasSuffix(str, "\n") {
		str += "\n"
	}
	t.terminal.Write([]byte(str))
}

// Printf is a wrapper to emulate the functionality of fmt.Printf and output
// to the SSH terminal.
func (t SSHTerminal) Printf(format string, a ...any) {
	str := fmt.Sprintf(format, a...)
	t.terminal.Write([]byte(str))
}

// ClientsByUser will get a list of clients this particular user has access to.
func ClientsByUser(user *pb.User) []*client.Client {
	cls := []*client.Client{}
	for i := range srv.clients {
		c := &srv.clients[i]
		for k := range c.Users {
			if user.Email == k.Email {
				cls = append(cls, c)
			}
		}
	}
	return cls
}

// Get the clients owned by this user
func MyClients(u *pb.User) []*client.Client {
	cls := []*client.Client{}
	for i := range srv.clients {
		c := &srv.clients[i]
		if c.Owner == u.Email {
			cls = append(cls, &srv.clients[i])
		}
	}
	return cls
}

// Get the clients who have access delegated to me
func MyDelegates(u *pb.User) []*client.Client {
	cls := []*client.Client{}
	for i := range srv.clients {
		c := &srv.clients[i]
		roles, ok := c.Users[u]
		if !ok {
			continue
		}
		for _, r := range roles {
			if r.Context == pb.Context_SSH {
				cls = append(cls, &srv.clients[i])
			}
		}
	}
	return cls
}

// User returns a user proto for the given email address
func User(email string) (*pb.User, error) {
	for i := range srv.users {
		if srv.users[i].Email == email {
			return srv.users[i], nil
		}
	}
	return nil, fmt.Errorf("User(%q): unable to locate user", email)
}

// Make a string red
func red(s string) string {
	return ansiCode{foreground: ColorRed}.Render() + s + AnsiReset
}

// make it green!
func green(s string) string {
	return ansiCode{foreground: ColorGreen}.Render() + s + AnsiReset
}

// make it yellow
func yellow(s string) string {
	return ansiCode{foreground: ColorYellow}.Render() + s + AnsiReset
}

func magenta(s string) string {
	return ansiCode{foreground: ColorMagenta}.Render() + s + AnsiReset
}

// MyServersResponse will format a string containing all the gameservers and
// states related to the logged in user. It shows servers they own first and
// then servers that have had access delegated to them.
func MyServersResponse(s ssh.Session) (string, error) {
	output := ""
	u, err := User(s.User())
	if err != nil {
		return "", err
	}
	mycls := MyClients(u)
	if len(mycls) > 0 {
		var status string
		output = "Your Servers:\n"
		for _, c := range mycls {
			status = ""
			if c.Connected && c.Trusted {
				status = fmt.Sprintf(" [%s]", green("connected"))
			} else {
				status = fmt.Sprintf(" [%s]", red("offline"))
			}
			output += fmt.Sprintf("  %-20s%s\n", c.Name, status)
		}
	}

	mydels := MyDelegates(u)
	if len(mydels) > 0 {
		var status string
		output = "Delegated Servers:\n"
		for _, c := range mydels {
			status = ""
			if c.Connected && c.Trusted {
				status = fmt.Sprintf(" [%s]", green("connected"))
			} else {
				status = fmt.Sprintf(" [%s]", red("offline"))
			}
			output += fmt.Sprintf("  %-20s%s\n", c.Name, status)
		}
	}
	return output, nil
}

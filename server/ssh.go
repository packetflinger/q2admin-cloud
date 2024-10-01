// An SSH server is used to provide interactive access to Quake 2 server operators
package server

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/packetflinger/q2admind/client"
	"golang.org/x/term"

	pb "github.com/packetflinger/q2admind/proto"
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
func startSSHServer() {
	s := &ssh.Server{
		Addr: fmt.Sprintf("%s:%d",
			srv.config.GetSshAddress(),
			srv.config.GetSshPort(),
		),
		Handler:         sessionHandler,
		PasswordHandler: passwordHandler,
	}
	log.Println("Listening for SSH connections on", s.Addr)
	log.Fatal(s.ListenAndServe())
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
	sshterm := SSHTerminal{terminal: term.NewTerminal(s, "> ")}
	/*
		cl, err := srv.FindClientByName("local-test")
		if err != nil {
			sshterm.Println(fmt.Sprintf("error: unable to locate %q", "local-test"))
			return
		}
		activeClient = cl
		sshterm.terminal.SetPrompt(cl.Name + "> ")
	*/

	// go linkClientToTerminal(activeClient, sshterm)

	for {
		line, err := sshterm.terminal.ReadLine()
		if err != nil {
			break
		}
		c, err := ParseCmdArgs(line)
		if err != nil {
			sshterm.Println(err.Error())
		}

		if c.command == "server" {
			msg := ""
			if c.argc == 0 {
				msg = "Available Q2 Servers:\n"
				for _, c := range srv.clients {
					if !(c.Connected && c.Trusted) {
						continue
					}
					msg += fmt.Sprintf("  %s\n", c.Name)
				}
			} else {
				cl, err = srv.FindClientByName(c.argv[0])
				if err != nil {
					sshterm.Println(fmt.Sprintf("server: unable to locate %q", c.argv[0]))
					continue
				}
				if cl.TermCount > 0 {
					cl.TermCount--
				}
				closeClientTerminalChannel(cl)
				activeClient = cl
				go linkClientToTerminal(activeClient, sshterm)
				sshterm.terminal.SetPrompt(cl.Name + "> ")
			}
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
			SayEveryone(cl, PRINT_CHAT, c.args)
		}
		if c.command == "help" {
			msg := "Available commands:\n"
			msg += "  help               - show this message\n"
			msg += "  mute <#> <secs>    - mute player # for secs seconds\n"
			msg += "  quit               - close the ssh connection\n"
			msg += "  rcon <cmd>         - execute <cmd> on the remote server\n"
			msg += "  say <text>         - broadcasts <text> to all players\n"
			msg += "  server [name]      - switch management servers\n"
			msg += "                       omitting [name] will list possible servers\n"
			msg += "  stuff <#> <cmd>    - force client # to do a command\n"
			msg += "  whois <#>          - show player info for client #\n"
			sshterm.Println(msg)
		}
		if c.command == "quit" || c.command == "exit" || c.command == "q" {
			closeClientTerminalChannel(cl)
			break
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
				msg := fmt.Sprintf("whois error: invalid player ID: %d", pid)
				sshterm.Println(msg)
				continue
			}
			p := activeClient.Players[pid]
			if p.ConnectTime == 0 {
				msg := fmt.Sprintf("whois: client_id %q not in use", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			msg := p.Dump()
			sshterm.Println(msg)
		}
		if c.command == "stuff" {
			if len(c.args) == 0 {
				activeClient.SSHPrintln("Usage: stuff <id> <command>")
				continue
			}
			pid, err := strconv.Atoi(c.argv[0])
			if err != nil {
				sshterm.Println("stuff error: " + err.Error())
				continue
			}
			if pid < 0 || pid > activeClient.MaxPlayers {
				msg := fmt.Sprintf("stuff error: invalid player ID: %d", pid)
				sshterm.Println(msg)
				continue
			}
			p := &activeClient.Players[pid]
			if p.ConnectTime == 0 {
				msg := fmt.Sprintf("stuff: client_id %q not in use", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			StuffPlayer(cl, p, strings.Join(c.argv[1:], " "))
		}
		if c.command == "rcon" {
			// this is not a real rcon command (out-of-band over UDP), just
			// simulated over existing TCP connection
			if len(c.args) == 0 {
				activeClient.SSHPrintln("Usage: rcon <command>")
				continue
			}
			ConsoleCommand(activeClient, c.args)
		}
		if c.command == "status" {
			str := activeClient.StatusString()
			activeClient.SSHPrintln(str)
		}
		if c.command == "consolesay" {
			if len(c.args) == 0 {
				sshterm.Println("Usage: consolesay <message>")
				continue
			}
			ConsoleSay(cl, c.args)
			cl.Log.Println("console:", c.args)
			activeClient.SSHPrintln("console: " + c.args)
		}
		if c.command == "sayperson" {
			if len(c.args) == 0 {
				sshterm.Println("Usage: sayplayer <id> [message]")
				continue
			}
			id, err := strconv.Atoi(c.argv[0])
			if err != nil {
				msg := fmt.Sprintf("sayplayer: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			if id < 0 || id > activeClient.MaxPlayers {
				msg := fmt.Sprintf("sayplayer: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			p := &activeClient.Players[id]
			if p.ConnectTime == 0 {
				msg := fmt.Sprintf("sayperson: client_id %q not in use", c.argv[0])
				activeClient.SSHPrintln(msg)
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
				msg := fmt.Sprintf("kick: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			if id < 0 || id > activeClient.MaxPlayers {
				msg := fmt.Sprintf("kick: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			p := &activeClient.Players[id]
			if p.ConnectTime == 0 {
				msg := fmt.Sprintf("kick: client_id %q not in use", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			KickPlayer(cl, p, strings.Join(c.argv[1:], " "))
		}
		if c.command == "mute" {
			if len(c.args) == 0 { // list all mutes
				activeClient.SSHPrintln("Active mutes:")
				details := ""
				for _, m := range activeClient.Rules {
					if m.Type != pb.RuleType_MUTE {
						continue
					}
					mtxt, err := RuleDetailLine(m)
					if err != nil {
						msg := fmt.Sprintf("mute list error: %v", err)
						activeClient.SSHPrintln("  " + msg)
						continue
					}
					details += mtxt + "\n"
				}
				sshterm.Println(details + "\nUsage: mute <player_id> <seconds>")
				continue
			}
			id, err := strconv.Atoi(c.argv[0])
			if err != nil {
				msg := fmt.Sprintf("mute: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			if id < 0 || id > activeClient.MaxPlayers {
				msg := fmt.Sprintf("mute: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
				continue
			}
			secs, err := strconv.Atoi(c.argv[1])
			if err != nil {
				msg := fmt.Sprintf("mute: invalid seconds %q", c.argv[1])
				activeClient.SSHPrintln(msg)
				continue
			}
			p := &activeClient.Players[id]
			if p.ConnectTime == 0 {
				msg := fmt.Sprintf("mute: client_id %q not in use", c.argv[0])
				activeClient.SSHPrintln(msg)
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
	}
}

// linkClientToTerminal connects the client to the the ssh terminal to receive and
// display console messages. This is one-way from Q2 server to terminal to
// show things like connects, disconnects, chats, and internal q2admin stuff.
//
// The ssh user can select which client to watch. This is run concurrently
// and is stopped when the user switches clients
func linkClientToTerminal(cl *client.Client, t SSHTerminal) {
	var now string
	var msg string
	t.Println("* linking terminal to " + cl.Name)
	if cl.TermCount == 0 {
		cl.TermLog = make(chan string)
	}
	cl.TermCount++
	for {
		logmsg, ok := <-cl.TermLog
		if !ok {
			break
		}
		now = time.Now().Format("15:04:05")
		msg = fmt.Sprintf("%s %s\n", now, logmsg)
		if cl.TermPaused {
			cl.TermBuf = append(cl.TermBuf, msg)
		} else {
			t.Println(msg)
		}
	}
	t.Println("* unlinking " + cl.Name)
}

func closeClientTerminalChannel(cl *client.Client) {
	if cl.TermCount == 0 && cl.TermLog != nil {
		close(cl.TermLog)
	}
}

// passwordHandler provides user/password-based authentication. Terminal access
// should normally be through the cloud admin website, so usernames/passwords
// should pass through from their website login. Username will be their email
// address, and the password is randomly generated at website login time.
//
// Since it's technically possible to expose this SSH server externally, having
// some kind of authentication method is necessary.
//
// Return true == allow connection, false == deny
func passwordHandler(ctx ssh.Context, password string) bool {
	return true
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

// An SSH server is used to provide interactive access to Quake 2 server operators
package server

import (
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
	var activeClient *client.Client
	sshterm := SSHTerminal{terminal: term.NewTerminal(s, "> ")}
	cl, err := srv.FindClientByName("local-test")
	if err != nil {
		sshterm.Println(fmt.Sprintf("error: unable to locate %q", "local-test"))
		return
	}
	activeClient = cl
	sshterm.terminal.SetPrompt(cl.Name + "> ")

	go linkClientToTerminal(activeClient, sshterm)

	for {
		line, err := sshterm.terminal.ReadLine()
		if err != nil {
			break
		}
		c, err := parseSSHCmd(line)
		if err != nil {
			sshterm.Println(err.Error())
			continue
		}
		if c.cmd == SSHCmdServer {
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
					sshterm.Println(fmt.Sprintf("error: unable to locate %q", c.argv[0]))
					continue
				}
				cl.TermCount--
				closeClientTerminalChannel(cl)
				activeClient = cl
				go linkClientToTerminal(activeClient, sshterm)
				sshterm.terminal.SetPrompt(cl.Name + "> ")
			}
			sshterm.Println(msg)
		}
		if c.cmd == SSHCmdSay {
			SayEveryone(cl, PRINT_CHAT, c.args)
		}
		if c.cmd == SSHCmdHelp {
			msg := "Available commands:\n"
			msg += "  help               - show this message\n"
			msg += "  quit               - close the ssh connection\n"
			msg += "  rcon <cmd>         - execute <cmd> on the remote server\n"
			msg += "  say <text>         - broadcasts <text> to all players\n"
			msg += "  server [name]      - switch management servers\n"
			msg += "                       omitting [name] will list possible servers\n"
			msg += "  stuff <#> <cmd>    - force client # to do a command\n"
			msg += "  whois <#>          - show player info for client #\n"
			sshterm.Println(msg)
		}
		if c.cmd == SSHCmdQuit {
			closeClientTerminalChannel(cl)
			break
		}
		if c.cmd == SSHCmdWhois {
			// todo: input validation
			pid, _ := strconv.Atoi(c.argv[0])
			p := activeClient.Players[pid]
			msg := p.Dump()
			sshterm.Println(msg)
		}
		if c.cmd == SSHCmdStuff {
			// todo: input validation
			pid, _ := strconv.Atoi(c.argv[0])
			p := &activeClient.Players[pid]
			StuffPlayer(cl, p, strings.Join(c.argv[1:], " "))
		}
		if c.cmd == SSHCmdRcon {
			// this is not a real rcon command (out-of-band over UDP), just
			// simulated over existing TCP connection
			if len(c.args) == 0 {
				activeClient.SSHPrintln("Usage: rcon <command>")
				return
			}
			ConsoleCommand(activeClient, c.args)
		}
		if c.cmd == SSHCmdStatus {
			str := activeClient.StatusString()
			activeClient.SSHPrintln(str)
		}
		if c.cmd == SSHCmdConsoleSay {
			ConsoleSay(cl, c.args)
			cl.Log.Println("console:", c.args)
			activeClient.SSHPrintln("console: " + c.args)
		}
		if c.cmd == SSHCmdSayPlayer {
			id, err := strconv.Atoi(c.argv[0])
			if err != nil {
				msg := fmt.Sprintf("sayplayer: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
			}
			if id < 0 || id > activeClient.MaxPlayers {
				msg := fmt.Sprintf("sayplayer: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
			}
			p := &activeClient.Players[id]
			SayPlayer(cl, p, PRINT_CHAT, strings.Join(c.argv[1:], " "))
		}
		if c.cmd == SSHCmdKick {
			id, err := strconv.Atoi(c.argv[0])
			if err != nil {
				msg := fmt.Sprintf("kick: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
			}
			if id < 0 || id > activeClient.MaxPlayers {
				msg := fmt.Sprintf("kick: invalid client_id %q", c.argv[0])
				activeClient.SSHPrintln(msg)
			}
			p := &activeClient.Players[id]
			KickPlayer(cl, p, strings.Join(c.argv[1:], " "))
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
		t.Println(now + " " + logmsg + "\n")
	}
	t.Println("* unlinking " + cl.Name)
}

func closeClientTerminalChannel(cl *client.Client) {
	if cl.TermCount == 0 {
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

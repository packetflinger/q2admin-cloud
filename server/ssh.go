// An SSH server is used to provide interactive access to Quake 2 server operators
package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/packetflinger/q2admind/client"
	"github.com/packetflinger/q2admind/database"
	"github.com/packetflinger/q2admind/util"
	"golang.org/x/term"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
	gossh "golang.org/x/crypto/ssh"
)

const (
	TopLevelPrompt = "q2a"
	PauseLength    = 600 // 10 minutes
)

const (
	TermMsgTypeGeneral = iota
	TermMsgTypePlayerChat
	TermMsgTypeBan
	TermMsgTypeMute
)

type CmdArgs struct {
	command string
	argc    int
	argv    []string
	args    string
}

type HelpCommands struct {
	Cmds []struct {
		Cmd  string
		Desc string
	}
	Extra string
}

type SearchResultsOutput struct {
	Query   string
	Results []database.SearchResult
}

const (
	helpTemplate = `
Available commands:
{{- range .Cmds}}
  {{ printf "%-20s" .Cmd }}                   {{ .Desc -}}
{{end}}

{{.Extra}}
`
	statusTemplate = `
Frontend:      {{ .IPAddress }}:{{ .Port }}
Peer:          {{ .Connection.RemoteAddr.String | magenta }}
Current map:   {{ .CurrentMap }}
Previous map:  {{ .PreviousMap }}
Invite Tokens: {{ .Invites.Tokens }}/{{ .Invites.Max }}
Invites Used:  {{ .Invites.UseCount }}
Teleports:     {{ .TeleportCount }}

{{ if .PlayerCount }}
num score name            vpn address
--- ----- --------------- --- ------------------------------
{{ range .Players}}
{{- if .IP -}}
{{ printf "%3d" .ClientID}} {{ printf "%5d" .Frags}} {{ printf "%-15s" .Name }}  {{ .VPN | checkMark | red }}  {{ .IP -}}
{{- end -}}
{{ end }}
{{ else }}
No UDP clients
{{ end }}
`

	searchTemplate = `
Search results for "{{ .Query }}"
name             server              seen  address
---------------  ---------------  -------  -------------------------
{{ range .Results -}}
{{ printf "%-15s" .Name }}  {{ printf "%-15s" .Server}}  {{ printf "%7s" .Ago }}  {{ .IP }}
{{ end }}
`
	rulesTemplate = `
id        type     description
--------  -------  -----------------------------------------------------
{{ range . -}}
{{ if .GetUuid }}{{ slice .GetUuid 0 8}}  {{ printf "%-7s" .GetType }}  {{ join .GetDescription " " | truncate 53 }}{{ end }}
{{ end }}
`

	whoisTemplate = `
Player information:
  name:     {{ .Name }}
  ip:       {{ .IP }} 
  dns:      {{ .Hostname }}
  client:   {{ .Version }}
  vpn:      {{ .VPN }}

UserInfo Data:
{{ range $k, $v := .UserinfoMap -}}
{{ printf "%15s" $k}} = {{ $v }}
{{ end -}}

Rules matching:
id        type     description
--------  -------  -----------------------------------------------------
{{ range .Rules -}}
{{ slice .GetUuid 0 8}}  {{ printf "%-7s" .GetType }}  {{ join .GetDescription " " | truncate 53 }}
{{ end }}
`
	serversTemplate = `
Your servers:
Name                  Status     Time  Peer
--------------------  ---------  ----  --------------------------------------
{{ range . -}}
{{ printf "%-20s" .Name }}  {{ printf "%-9s" (. | connected) }}  {{ if .Connection }}{{ printf "%-4s" (.ConnectTime | ago)}}  {{ .Connection.RemoteAddr.String }}{{ end }}
{{ end -}}
`
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
	sshterm := SSHTerminal{terminal: term.NewTerminal(s, "> ")}
	sshterm.SetPrompt(TopLevelPrompt, true)

	funcmap := template.FuncMap{
		"join": strings.Join,
		"truncate": func(s int, str string) string {
			if len(str) > s {
				return str[0:s]
			}
			return str
		},
		"green":     green,
		"red":       red,
		"yello":     yellow,
		"magenta":   magenta,
		"checkMark": checkMark,
		"connected": connectionIndicator,
		"now":       time.Now().Unix,
		"ago":       util.TimeAgo,
	}

	helpTmpl := template.Must(template.New("helpout").Parse(helpTemplate))
	statusTmpl := template.Must(template.New("statusout").Funcs(funcmap).Parse(statusTemplate))
	searchTmpl := template.Must(template.New("searchout").Parse(searchTemplate))
	rulesTmpl := template.Must(template.New("rulesout").Funcs(funcmap).Parse(rulesTemplate))
	whoisTmpl := template.Must(template.New("whoisout").Funcs(funcmap).Parse(whoisTemplate))
	srvTmpl := template.Must(template.New("srvout").Funcs(funcmap).Parse(serversTemplate))

	for {
		line, err := sshterm.terminal.ReadLine()
		if err != nil {
			break
		}
		if activeClient != nil && !activeClient.Connected { // server dropped
			sshterm.Printf(yellow("** server connection to %s dropped **\n"), activeClient.Name)
			if cancel != nil {
				cancel()
			}
			sshterm.SetPrompt(TopLevelPrompt, true)
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
				u, err := User(s.User())
				if err != nil {
					sshterm.Println("problems identifying you...")
					continue
				}
				var msg bytes.Buffer
				cls := MyClients(u)
				if err := srvTmpl.Execute(&msg, cls); err != nil {
					log.Println("error executing servers template:", err)
				}
				sshterm.Println(msg.String())
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
				activeClient = cl
				ctx, cancel = context.WithCancel(context.Background())

				newterm := make(chan string)
				cl.Terminals = append(cl.Terminals, &newterm)

				go linkClientToTerminal(ctx, activeClient, &sshterm, &newterm)
				defer cancel()

				sshterm.SetPrompt(fmt.Sprintf("%s/%s", TopLevelPrompt, cl.Name), true)
			}
			sshterm.Println(msg)

		} else if c.command == "quit" || c.command == "exit" || c.command == "logout" || c.command == "q" {
			break

		} else if (c.command == "help" || c.command == "?") && activeClient == nil {
			help := HelpCommands{
				Cmds: []struct {
					Cmd  string
					Desc string
				}{
					{Cmd: "help", Desc: "show this message"},
					{Cmd: "quit", Desc: "close the ssh connection"},
					{Cmd: "server [name]", Desc: "switch mgmt servers, list"},
				},
				Extra: "You need to use the server command to connect to a management server",
			}

			var msg bytes.Buffer
			if err := helpTmpl.Execute(&msg, help); err != nil {
				log.Println("error executing help command template:", err)
			}
			sshterm.Println(msg.String())
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

		} else if c.command == "help" || c.command == "?" {
			help := HelpCommands{
				Cmds: []struct {
					Cmd  string
					Desc string
				}{
					{Cmd: "help", Desc: "show this message"},
					{Cmd: "quit", Desc: "close the ssh connection"},
					{Cmd: "pause", Desc: "pause the console stream"},
					{Cmd: "server [name]", Desc: "switch mgmt servers, list"},
					{Cmd: "settings", Desc: "show front-end config"},
					{Cmd: "unpause", Desc: "resume the console stream"},
					{Cmd: "", Desc: ""},
					{Cmd: "rcon <cmd>", Desc: "execute <cmd> on the remote server"},
					{Cmd: "status", Desc: "display basic server status info"},
					{Cmd: "search <string>", Desc: "search player records (names, hosts, userinfo, etc)"},
					{Cmd: "stuff <#> <cmd>", Desc: "force client # to do a command"},
					{Cmd: "whois <#>", Desc: "show player info for client #"},
					{Cmd: "", Desc: ""},
					{Cmd: "say", Desc: "broadcasts <text> to all players"},
					{Cmd: "consolesay", Desc: "send print to server from console"},
					{Cmd: "sayplayer <id> <msg>", Desc: "say something to player #id"},
					{Cmd: "", Desc: ""},
					{Cmd: "kick <#> [msg]", Desc: "kick player # with msg"},
					{Cmd: "mute <#> <secs>", Desc: "mute player # for secs seconds"},
				},
			}
			var msg bytes.Buffer
			if err := helpTmpl.Execute(&msg, help); err != nil {
				log.Println("error executing extended help command template:", err)
			}
			sshterm.Println(msg.String())

		} else if c.command == "whois" {
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
			var msg bytes.Buffer
			if err := whoisTmpl.Execute(&msg, p); err != nil {
				log.Println("error executing whois template:", err)
			}
			sshterm.Println(msg.String())

		} else if c.command == "stuff" {
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

		} else if c.command == "rcon" {
			// this is not a real rcon command (out-of-band over UDP), just
			// simulated over existing TCP connection
			if len(c.args) == 0 {
				sshterm.Println("Usage: rcon <command>")
				continue
			}
			ConsoleCommand(activeClient, c.args)

		} else if c.command == "status" {
			var msg bytes.Buffer
			if err := statusTmpl.Execute(&msg, cl); err != nil {
				log.Println("error executing status command template:", err)
			}
			sshterm.Println(msg.String())

		} else if c.command == "consolesay" {
			if len(c.args) == 0 {
				sshterm.Println("Usage: consolesay <message>")
				continue
			}
			ConsoleSay(cl, c.args)
			cl.Log.Println("console:", c.args)
			sshterm.Println("console: " + c.args)

		} else if c.command == "sayperson" {
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

		} else if c.command == "kick" {
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

		} else if c.command == "mute" {
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

		} else if c.command == "pause" {
			if sshterm.paused > 0 {
				continue
			}
			sshterm.paused = time.Now().Unix() + PauseLength
			sshterm.SetPrompt(fmt.Sprintf("%s [%s]", sshterm.prompt, red("paused")), false)

		} else if c.command == "unpause" {
			if sshterm.paused == 0 {
				continue
			}
			sshterm.paused = 0
			for _, line := range sshterm.buffer {
				sshterm.Println(line)
			}
			sshterm.RestorePrompt()
			sshterm.buffer = nil

		} else if c.command == "search" {
			if c.argc == 0 {
				sshterm.Println("Usage: search <partial_name_ip_host_userinfo>")
				continue
			}
			res, err := db.Search(c.args)
			if err != nil {
				sshterm.Printf("database.Search(%q): %v\n", c.args, err)
				continue
			}

			var msg bytes.Buffer
			so := SearchResultsOutput{
				Query:   c.args,
				Results: res,
			}
			if err := searchTmpl.Execute(&msg, so); err != nil {
				log.Println("error executing search template:", err)
			}
			sshterm.Println(msg.String())
			continue
		} else if c.command == "rules" {
			if c.argc == 0 {
				sshterm.Printf("CLIENT-level rules in affect on %q:\n", cl.Name)
				var msg bytes.Buffer
				if err := rulesTmpl.Execute(&msg, cl.Rules); err != nil {
					log.Println("error executing rules template:", err)
				}
				sshterm.Println(msg.String())
				msg.Reset()
				sshterm.Printf("SERVER-level rules in affect on %q:\n", cl.Name)
				if err := rulesTmpl.Execute(&msg, srv.rules); err != nil {
					log.Println("error executing rules template:", err)
				}
				sshterm.Println(msg.String())
			} else if c.argc > 1 && c.argv[0] == "show" {
				for _, r := range append(cl.Rules, srv.rules...) {
					if strings.HasPrefix(r.GetUuid(), c.argv[1]) {
						sshterm.Printf("Detail for rule [%s]:\n\n", c.argv[1])
						sshterm.Println(prototext.Format(r))
						break
					}
				}
			} else if c.argc > 1 && (c.argv[0] == "del" || c.argv[0] == "delete" || c.argv[0] == "remove") {
				id := c.argv[1]
				if len(id) < 4 {
					sshterm.Printf("ID %q has too few characters\n", id)
					continue
				}
				notAllowed := false
				for _, r := range srv.rules {
					if strings.HasPrefix(r.Uuid, id) {
						sshterm.Printf("Rule %q is applied globally, you can't remove it\n", r.Uuid)
						notAllowed = true
						break
					}
				}
				if notAllowed {
					continue
				}
				var found *pb.Rule
				var newrules []*pb.Rule
				for i, r := range cl.Rules {
					if strings.HasPrefix(r.Uuid, id) {
						found = r
						continue
					}
					newrules = append(newrules, cl.Rules[i])
				}
				if found == nil {
					sshterm.Printf("error: no client-scoped rule found with ID %q\n", id)
					continue
				}
				cl.Rules = newrules
				err = cl.MaterializeRules(cl.Rules)
				if err != nil {
					log.Println(err)
					sshterm.Println("error writing rules to persistent storage")
					continue
				}
				sshterm.Printf("Rule %q removed.\n", found.Uuid)
			} else if c.argc == 1 && c.argv[0] == "add" {
				r, err := AddRuleWizard(&sshterm, cl)
				if err != nil {
					sshterm.Println(err.Error())
					continue
				}
				sshterm.Printf("Adding rule proto:\n%s\n", prototext.Format(r))
				cl.Rules = append(cl.Rules, r)
				err = cl.MaterializeRules(cl.Rules)
				if err != nil {
					sshterm.Printf("%s", err.Error())
				}
			}
		} else if c.command == "settings" {
			sshterm.Printf("%s\n", prototext.Format(activeClient.ToProto()))
		}
		SendMessages(cl)
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
func linkClientToTerminal(ctx context.Context, cl *client.Client, t *SSHTerminal, stream *chan string) {
	if cl == nil || t == nil {
		return
	}
	var now, msg string
	msg = fmt.Sprintf("* connecting to %s's console stream *", cl.Name)
	t.Println(yellow(msg))

	for {
		select {
		case srvmsg := <-*stream:
			now = time.Now().Format("15:04:05")
			msg = fmt.Sprintf("%s %s\n", now, srvmsg)
			if t.paused > 0 {
				t.buffer = append(t.buffer, msg)
			} else {
				t.Println(msg)
			}
		case <-ctx.Done(): // cancel() called from SSH thread
			cl.Terminals = cl.TerminalDisconnected(stream)
			return
		}
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

// ParseCmdArgs breaks up the current SSH command and args
func ParseCmdArgs(input string) (CmdArgs, error) {
	if input == "" {
		return CmdArgs{}, nil
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
	if str == "" {
		return
	}
	if !strings.HasSuffix(str, "\n") {
		str += "\n"
	}
	t.terminal.Write([]byte(str))
}

// Printf is a wrapper to emulate the functionality of fmt.Printf and output
// to the SSH terminal.
func (t SSHTerminal) Printf(format string, a ...any) {
	if format == "" {
		return
	}
	str := fmt.Sprintf(format, a...)
	t.terminal.Write([]byte(str))
}

// ClientsByUser will get a list of clients this particular user has access to.
func ClientsByUser(user *pb.User) []*client.Client {
	var cls []*client.Client
	if user == nil {
		return cls
	}
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
	var cls []*client.Client
	if u == nil {
		return cls
	}
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
	var cls []*client.Client
	if u == nil {
		return cls
	}
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
	if email == "" {
		return nil, fmt.Errorf("blank email input")
	}
	for i := range srv.users {
		if srv.users[i].Email == email {
			return srv.users[i], nil
		}
	}
	return nil, fmt.Errorf("User(%q): unable to locate user", email)
}

// Convert a logical variable (1/0, true/false, "yes"/"no") into an emoji
// checkmark
func checkMark(in any) string {
	if reflect.TypeOf(in) == reflect.TypeOf(true) {
		if in == true {
			return "\u2713"
		}
	}
	if reflect.TypeOf(in) == reflect.TypeOf(1) {
		if in == 1 {
			return "\u2713"
		}
	}
	return " "
}

// SetPrompt will set the current terminal's prompt to the s arg. The save arg
// will cause the terminal to keep a local copy of the prompt. This will allow
// for restoring it back to a previous value after a temporary change.
//
// The "> " is appended to the end when set, don't include that manually.
func (t *SSHTerminal) SetPrompt(s string, save bool) {
	if s == "" {
		return
	}
	if save {
		t.prompt = s
	}
	t.terminal.SetPrompt(s + "> ")
}

// RestorePrompt will change the prompt back to whatever value is set in the
// `prompt` property. This is only useful if SetPrompt() is used with the
// `save` property as false.
func (t *SSHTerminal) RestorePrompt() {
	if t.prompt == "" {
		t.prompt = "> "
	}
	t.SetPrompt(t.prompt, false)
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

// AddRuleWizard will prompt the user to enter all the data needed to construct
// a rule proto affecting players.
func AddRuleWizard(t *SSHTerminal, cl *client.Client) (*pb.Rule, error) {
	if t == nil || cl == nil {
		return nil, fmt.Errorf("null terminal or client")
	}
	var r pb.Rule
	t.SetPrompt("", false)
gettype:
	t.Printf("  [Rule Wizard] What type of rule to create? (ban, mute, stifle, message)? ")
	in, err := t.terminal.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("error reading rule wizard input: %v", err)
	}
	if in != "ban" && in != "mute" && in != "stifle" && in != "message" {
		t.Println("Invalid selection")
		goto gettype
	}
	switch in {
	case "ban":
		r.Type = pb.RuleType_BAN
	case "mute":
		r.Type = pb.RuleType_MUTE
	case "stifle":
		r.Type = pb.RuleType_STIFLE
	case "message":
		r.Type = pb.RuleType_MESSAGE
	}

	t.Printf("  [Rule Wizard] What network address should this affect (CIDR notation (8.8.8.0/24))? ")
	in, err = t.terminal.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("error reading rule wizard input: %v", err)
	}
	r.Address = append(r.Address, in)

	t.Printf("  [Rule Wizard] Enter a description for this rule (only admins can see this): ")
	in, err = t.terminal.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("error reading rule wizard input: %v", err)
	}
	r.Description = append(r.Description, in)

	t.Printf("  [Rule Wizard] Enter a message to display to players matching this rule: ")
	in, err = t.terminal.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("error reading rule wizard input: %v", err)
	}
	r.Message = append(r.Message, in)
	r.Uuid = uuid.NewString()
	t.RestorePrompt()
	return &r, nil
}

// Helper func for using in templates
func connectionIndicator(c *client.Client) string {
	if c == nil {
		return "error"
	}
	if c.Connected && c.Trusted {
		return green("connected")
	}
	return red("offline")
}

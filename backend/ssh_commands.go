// SSH commands: parsing a command line and running each command.

package backend

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

type CmdArgs struct {
	command string
	argc    int
	argv    []string
	args    string
}

// ParseCmdArgs breaks up the current SSH command and args. This allows for the
// typical `argc`, `argv`, `args` functionality to make it easier to understand
// what the user is actually trying to do.
func ParseCmdArgs(input string) (CmdArgs, error) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return CmdArgs{}, nil
	}
	return CmdArgs{
		command: strings.ToLower(tokens[0]),
		argc:    len(tokens) - 1,
		argv:    tokens[1:],
		args:    strings.Join(tokens[1:], " "),
	}, nil
}

// resolvePlayer finds the single connected player on fe matching in, which is
// a client ID or part of a name.
func resolvePlayer(fe *frontend.Frontend, in string) (*frontend.Player, error) {
	players, err := fe.ResolvePlayers(in)
	if err != nil {
		return nil, err
	}
	if len(players) != 1 {
		return nil, fmt.Errorf("%q doesn't match exactly 1 player", in)
	}
	return players[0], nil
}

// cmdServer lists the user's servers, or with a name, switches to managing
// that one and streams its console.
func (ss *sshSession) cmdServer(c CmdArgs) {
	u, err := User(ss.s.User())
	if err != nil {
		ss.term.Println("problems identifying you...")
		return
	}
	if c.argc == 0 {
		var msg bytes.Buffer
		cls := FrontendsByIdentity(u.GetEmail())
		if err := srvTmpl.Execute(&msg, cls); err != nil {
			log.Println("error executing servers template:", err)
		}
		ss.term.Println(msg.String())
		return
	}
	fe, err := be.FindFrontendByName(c.argv[0])
	// same message for unknown and unauthorized so names can't be probed
	if err != nil || !canManage(u, fe) {
		ss.term.Printf("server: unable to locate %q\n", c.argv[0])
		return
	}
	if !(fe.Connected && fe.Trusted) {
		ss.term.Printf("%q is offline, it can't be managed currently\n", c.argv[0])
		return
	}
	ss.cancel() // detach from the previous server's stream
	ss.active = fe
	ctx, stop := context.WithCancel(ss.s.Context())
	ss.cancel = stop

	// buffered: ConsoleBroadcast drops messages when this is full
	newterm := make(chan frontend.ConsoleLine, 64)
	backlog := fe.AddTerminal(&newterm, 0)

	go linkFrontendToTerminal(ctx, ss.active, ss.term, &newterm, backlog)

	ss.term.SetPrompt(fmt.Sprintf("%s/%s", TopLevelPrompt, fe.Name), true)
}

// cmdGroup lists and manages the user's server groups (see sshGroupCommand).
func (ss *sshSession) cmdGroup(c CmdArgs) {
	u, err := User(ss.s.User())
	if err != nil {
		ss.term.Println("problems identifying you...")
		return
	}
	ss.term.Println(sshGroupCommand(u.GetEmail(), c))
}

// cmdTopHelp lists the commands that work before a server is selected.
func (ss *sshSession) cmdTopHelp() {
	help := HelpCommands{
		Cmds: []struct {
			Cmd  string
			Desc string
		}{
			{Cmd: "help", Desc: "show this message"},
			{Cmd: "quit", Desc: "close the ssh connection"},
			{Cmd: "server [name]", Desc: "switch mgmt servers, list"},
			{Cmd: "groups", Desc: "list and manage groups of servers (group help)"},
		},
		Extra: "You need to use the server command to connect to a management server",
	}

	var msg bytes.Buffer
	if err := helpTmpl.Execute(&msg, help); err != nil {
		log.Println("error executing help command template:", err)
	}
	ss.term.Println(msg.String())
}

// cmdHelp lists every command, for when a server is selected.
func (ss *sshSession) cmdHelp() {
	help := HelpCommands{
		Cmds: []struct {
			Cmd  string
			Desc string
		}{
			{Cmd: "help", Desc: "show this message"},
			{Cmd: "quit", Desc: "close the ssh connection"},
			{Cmd: "pause", Desc: "pause the console stream"},
			{Cmd: "server [name]", Desc: "switch mgmt servers, list"},
			{Cmd: "groups", Desc: "list and manage groups of servers (group help)"},
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
			{Cmd: "sayperson <id> <msg>", Desc: "say something to player #id"},
			{Cmd: "", Desc: ""},
			{Cmd: "kick <#> [msg]", Desc: "kick player # with msg"},
			{Cmd: "mute <#> <secs>", Desc: "mute player # for secs seconds"},
			{Cmd: "stifle <#> <secs>", Desc: "stifle player # for secs seconds"},
			{Cmd: "", Desc: ""},
			{Cmd: "rules [show <id> | del <id> | add]", Desc: ""},
		},
	}
	var msg bytes.Buffer
	if err := helpTmpl.Execute(&msg, help); err != nil {
		log.Println("error executing extended help command template:", err)
	}
	ss.term.Println(msg.String())
}

func (ss *sshSession) cmdSay(c CmdArgs) {
	if c.argc == 0 {
		ss.term.Println("Usage: say <something_to_say>")
		return
	}
	ss.term.Println(magenta(c.args))
	SayEveryone(ss.active, PRINT_CHAT, c.args)
}

func (ss *sshSession) cmdWhois(c CmdArgs) {
	if len(c.args) == 0 {
		ss.term.Println("Usage: whois <id>")
		return
	}
	p, err := resolvePlayer(ss.active, c.argv[0])
	if err != nil {
		ss.term.Printf("whois: %v\n", err)
		return
	}
	var msg bytes.Buffer
	if err := whoisTmpl.Execute(&msg, p); err != nil {
		log.Println("error executing whois template:", err)
	}
	ss.term.Println(msg.String())
}

func (ss *sshSession) cmdStuff(c CmdArgs) {
	if len(c.args) == 0 {
		ss.term.Println("Usage: stuff <id> <command>")
		return
	}
	p, err := resolvePlayer(ss.active, c.argv[0])
	if err != nil {
		ss.term.Printf("stuff: %v\n", err)
		return
	}
	StuffPlayer(ss.active, p, strings.Join(c.argv[1:], " "))
}

// cmdRcon runs a console command on the server. This is not a real rcon
// command (out-of-band over UDP), just simulated over the existing TCP
// connection.
func (ss *sshSession) cmdRcon(c CmdArgs) {
	if len(c.args) == 0 {
		ss.term.Println("Usage: rcon <command>")
		return
	}
	ConsoleCommand(ss.active, c.args)
}

func (ss *sshSession) cmdStatus() {
	var msg bytes.Buffer
	if err := statusTmpl.Execute(&msg, ss.active); err != nil {
		log.Println("error executing status command template:", err)
	}
	ss.term.Println(msg.String())
}

func (ss *sshSession) cmdConsoleSay(c CmdArgs) {
	if len(c.args) == 0 {
		ss.term.Println("Usage: consolesay <message>")
		return
	}
	ConsoleSay(ss.active, c.args)
	ss.active.Log.Println("console:", c.args)
	ss.term.Println("console: " + c.args)
}

func (ss *sshSession) cmdSayPerson(c CmdArgs) {
	if len(c.args) == 0 {
		ss.term.Println("Usage: sayperson <id> [message]")
		return
	}
	p, err := resolvePlayer(ss.active, c.argv[0])
	if err != nil {
		ss.term.Printf("sayperson: %v\n", err)
		return
	}
	SayPlayer(ss.active, p, PRINT_CHAT, strings.Join(c.argv[1:], " "))
}

func (ss *sshSession) cmdKick(c CmdArgs) {
	if len(c.args) == 0 {
		ss.term.Println("Usage: kick <id> [message]")
		return
	}
	p, err := resolvePlayer(ss.active, c.argv[0])
	if err != nil {
		ss.term.Printf("kick: %v\n", err)
		return
	}
	KickPlayer(ss.active, p, strings.Join(c.argv[1:], " "))
}

// cmdMute mutes a player, or with no arguments lists the active mutes.
func (ss *sshSession) cmdMute(c CmdArgs) {
	if len(c.args) == 0 { // list all mutes
		ss.term.Println("Active mutes:")
		details := ""
		for _, m := range ss.active.Rules {
			if m.Type != pb.RuleType_MUTE {
				continue
			}
			mtxt, err := RuleDetailLine(m)
			if err != nil {
				ss.term.Printf(" mute list error: %v\n", err)
				continue
			}
			details += mtxt + "\n"
		}
		ss.term.Println(details + "\nUsage: mute <player_id> <seconds>")
		return
	}
	if c.argc < 2 {
		ss.term.Println("Usage: mute <player_id> <seconds>")
		return
	}
	p, err := resolvePlayer(ss.active, c.argv[0])
	if err != nil {
		ss.term.Printf("mute: %v\n", err)
		return
	}
	secs, err := strconv.Atoi(c.argv[1])
	if err != nil {
		ss.term.Printf("mute: invalid seconds %q\n", c.argv[1])
		return
	}
	MutePlayer(ss.active, p, secs)
}

// cmdStifle stifles a player, or with no arguments lists the active stifles.
func (ss *sshSession) cmdStifle(c CmdArgs) {
	if len(c.args) == 0 { // list all stifles
		ss.term.Println("Active stifles:")

		var details strings.Builder
		for _, m := range ss.active.Rules {
			if m.Type != pb.RuleType_STIFLE {
				continue
			}
			mtxt, err := RuleDetailLine(m)
			if err != nil {
				ss.term.Printf(" stifle list error: %v\n", err)
				continue
			}
			_, err = details.WriteString(mtxt + "\n")
			if err != nil {
				ss.term.Printf("stifle list error: %v\n", err)
				continue
			}
		}
		ss.term.Println(details.String() + "\nUsage: stifle <player_id> <seconds>")
		return
	}
	if c.argc < 2 {
		ss.term.Println("Usage: stifle <player_id> <seconds>")
		return
	}
	p, err := resolvePlayer(ss.active, c.argv[0])
	if err != nil {
		ss.term.Printf("stifle: %v\n", err)
		return
	}
	secs, err := strconv.Atoi(c.argv[1])
	if err != nil {
		ss.term.Printf("stifle: invalid seconds %q\n", c.argv[1])
		return
	}
	StiflePlayer(ss.active, p, secs)
}

// cmdPause stops the terminal from scrolling with in-game messages (frags,
// chat, etc). The pause will automatically expire after a default of 10
// minutes. Msgs are buffered while paused.
func (ss *sshSession) cmdPause() {
	ss.term.mu.Lock()
	if ss.term.paused == 0 {
		ss.term.paused = time.Now().Unix() + PauseLength
		ss.term.SetPrompt(fmt.Sprintf("%s [%s]", ss.term.prompt, red("paused")), false)
	}
	ss.term.mu.Unlock()
}

func (ss *sshSession) cmdSearch(c CmdArgs) {
	if c.argc == 0 {
		ss.term.Println("Usage: search <partial_name_ip_host_userinfo>")
		return
	}
	res, err := db.Search(c.args)
	if err != nil {
		ss.term.Printf("database.Search(%q): %v\n", c.args, err)
		return
	}

	var msg bytes.Buffer
	so := SearchResultsOutput{
		Query:   c.args,
		Results: res,
	}
	if err := searchTmpl.Execute(&msg, so); err != nil {
		log.Println("error executing search template:", err)
	}
	ss.term.Println(msg.String())
}

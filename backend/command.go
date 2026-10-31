package backend

import (
	"bytes"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/packetflinger/q2admind/frontend"
)

// Have client broadcast print from "console"
func ConsoleSay(cl *frontend.Frontend, print string) {
	if cl == nil || print == "" {
		return
	}
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(fmt.Sprintf("say %s\n", print))
}

// Force a player to do a command
func StuffPlayer(cl *frontend.Frontend, p *frontend.Player, cmd string) {
	if cl == nil || p == nil || cmd == "" {
		return
	}
	stuffcmd := fmt.Sprintf("sv !stuff CL %d %s\n", p.ClientID, cmd)
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(stuffcmd)
}

// osc8 is text as a terminal hyperlink to url, for console messages. Terminals
// without hyperlink support just show the text; the web console only follows
// links to paths on its own site.
func osc8(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// consoleLink is fe's name linked to its web console. The console page checks
// the viewer's access.
func consoleLink(fe *frontend.Frontend) string {
	return osc8(path.Join("/sv", fe.UUID, fe.Name, "console"), fe.Name)
}

// playerLink is text (usually p's name) linked to p's player page on fe, then
// padded with spaces to width visible characters so columns still line up.
// The link is to the player's client slot, so it shows whoever is in that slot
// when it's opened.
func playerLink(fe *frontend.Frontend, p *frontend.Player, text string, width int) string {
	url := path.Join("/sv", fe.UUID, fe.Name, "player", strconv.Itoa(p.ClientID))
	return osc8(url, text) + strings.Repeat(" ", max(width-utf8.RuneCountInString(text), 0))
}

// linkChatName links the speaker's name at the start of a chat line ("name:
// text") when exactly one player has that name; otherwise the line is left as
// it is.
func linkChatName(fe *frontend.Frontend, line string) string {
	players, err := fe.GetPlayerFromPrint(line)
	if err != nil || len(players) != 1 || !strings.HasPrefix(line, players[0].Name+": ") {
		return line
	}
	p := players[0]
	return playerLink(fe, p, p.Name, 0) + line[len(p.Name):]
}

// Prevent the player from talking.
//
// Specify the length of silence using the seconds arg. Using zero or a
// negative number of seconds makes it permanent.
func MutePlayer(cl *frontend.Frontend, p *frontend.Player, seconds int) {
	var cmd string
	if cl == nil || p == nil {
		return
	}
	length := "perm"
	if seconds > 0 {
		cmd = fmt.Sprintf("sv !mute CL %d %d\n", p.ClientID, seconds)
		length = strconv.Itoa(seconds)
	} else {
		cmd = fmt.Sprintf("sv !mute CL %d PERM\n", p.ClientID)
	}
	line := func(name string) string { return fmt.Sprintf("MUTE[%s] %s [%d]\n", length, name, p.ClientID) }
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(cmd)
	cl.Log.Printf("%s", line(fmt.Sprintf("%-20s", p.Name)))
	cl.ConsoleBroadcast(line(playerLink(cl, p, p.Name, 20)))
}

// Throttle the player's talking.
//
// A stifle is a repeating temporary mute. A stifled player will be able to
// speak once, then will be muted for seconds amount of time. Then they'll
// be able to speak again, once followed by another period of silence.
//
// Seconds must be greater than 0, maximum length is 300 (5 minutes)
func StiflePlayer(cl *frontend.Frontend, p *frontend.Player, seconds int) {
	var cmd string
	if cl == nil || p == nil {
		return
	}
	if seconds < 0 {
		seconds = int(math.Abs(float64(seconds)))
	}
	if seconds > StifleMax {
		seconds = StifleMax
	}
	msg := "You've been stifled"
	cmd = fmt.Sprintf("sv !stifle CL %d %d\n", p.ClientID, seconds)
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(cmd)
	(&cl.MessageOut).WriteByte(SCMDSayClient)
	(&cl.MessageOut).WriteByte(p.ClientID)
	(&cl.MessageOut).WriteByte(PRINT_HIGH)
	(&cl.MessageOut).WriteString(msg)

	line := func(name string) string { return fmt.Sprintf("STIFLE[%d] %s [%d]\n", seconds, name, p.ClientID) }
	cl.Log.Printf("%s", line(fmt.Sprintf("%-20s", p.Name)))
	cl.ConsoleBroadcast(line(playerLink(cl, p, p.Name, 20)))
}

// Instruct a client to kick a player. The target player will receive a direct
// message explaining why (if `msg` is not empty) just before the kick.
func KickPlayer(cl *frontend.Frontend, p *frontend.Player, msg string) {
	if cl == nil || p == nil {
		return
	}
	if msg != "" {
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		(&cl.MessageOut).WriteByte(SCMDSayClient)
		(&cl.MessageOut).WriteByte(p.ClientID)
		(&cl.MessageOut).WriteByte(PRINT_CHAT)
		(&cl.MessageOut).WriteString(msg)
	}
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(fmt.Sprintf("kick %d\n", p.ClientID))

	line := func(name string) string { return fmt.Sprintf("KICK %s [%d] %q\n", name, p.ClientID, msg) }
	cl.Log.Println(line(fmt.Sprintf("%-20s", p.Name)))
	cl.ConsoleBroadcast(line(playerLink(cl, p, p.Name, 20)))
}

// Issue a command as if you were typing it into the console. The command name
// is lowercased (see LowerCommandName).
//
// The engine prints the results of some commands only on the server's own
// console, outside the game library, so q2admin never sees them to send back.
// For those the console watchers get our own version instead:
//   - "say" is echoed as the "console: <text>" chat line players get
//   - "status" isn't sent at all; the watchers get the SSH status report
func ConsoleCommand(cl *frontend.Frontend, cmd string) {
	if cl == nil || cmd == "" {
		return
	}
	cmd = LowerCommandName(strings.TrimSuffix(cmd, "\n"))
	name, text, _ := strings.Cut(cmd, " ")
	if name == "status" {
		var out bytes.Buffer
		if err := statusTmpl.Execute(&out, cl); err != nil {
			cl.ConsoleBroadcast("status: " + err.Error())
			return
		}
		cl.ConsoleBroadcast(strings.Trim(out.String(), "\n"))
		return
	}
	if name == "say" && text != "" {
		// like the engine, drop one pair of quotes around the text
		if len(text) > 1 && strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) {
			text = text[1 : len(text)-1]
		}
		cl.ConsoleBroadcast(Font([]int{ColorGreen, WeightBold}, "console: "+text))
	}
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(cmd + "\n")
}

// LowerCommandName lowercases a console command's first word, leaving its
// arguments alone: "Say Hi There" becomes "say Hi There". Quake 2 command and
// cvar names are all lowercase, and phone keyboards capitalize the first word.
func LowerCommandName(cmd string) string {
	name, args, found := strings.Cut(cmd, " ")
	if !found {
		return strings.ToLower(cmd)
	}
	return strings.ToLower(name) + " " + args
}

// Send a message to every player on the server
func SayEveryone(cl *frontend.Frontend, level int, text string) {
	if cl == nil || text == "" {
		return
	}
	if level < 0 || level > PRINT_CHAT {
		level = PRINT_LOW
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	(&cl.MessageOut).WriteByte(SCMDSayAll)
	(&cl.MessageOut).WriteByte(level)
	(&cl.MessageOut).WriteString(text)
}

// Send a message to a particular player. Newlines automatically added.
func SayPlayer(cl *frontend.Frontend, p *frontend.Player, level int, text string) {
	if cl == nil || p == nil || text == "" {
		return
	}
	if level < 0 || level > PRINT_CHAT {
		level = PRINT_LOW
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	(&cl.MessageOut).WriteByte(SCMDSayClient)
	(&cl.MessageOut).WriteByte(p.ClientID)
	(&cl.MessageOut).WriteByte(level)
	(&cl.MessageOut).WriteString(text)
}

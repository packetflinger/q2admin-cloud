package backend

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/packetflinger/q2admind/crypto"
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

// Prevent the player from talking.
//
// Specify the length of silence using the seconds arg. Using zero or a
// negative number of seconds makes it permanent.
func MutePlayer(cl *frontend.Frontend, p *frontend.Player, seconds int) {
	var cmd, logMsg string
	if cl == nil || p == nil {
		return
	}
	if seconds > 0 {
		cmd = fmt.Sprintf("sv !mute CL %d %d\n", p.ClientID, seconds)
		logMsg = fmt.Sprintf("MUTE[%d] %-20s [%d]\n", seconds, p.Name, p.ClientID)
	} else {
		cmd = fmt.Sprintf("sv !mute CL %d PERM\n", p.ClientID)
		logMsg = fmt.Sprintf("MUTE[perm] %-20s [%d]\n", p.Name, p.ClientID)
	}
	(&cl.MessageOut).WriteByte(SCMDCommand)
	(&cl.MessageOut).WriteString(cmd)
	cl.Log.Printf("%s", logMsg)
	cl.ConsoleBroadcast(logMsg)
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

	logMsg := fmt.Sprintf("STIFLE[%d] %-20s [%d]\n", seconds, p.Name, p.ClientID)
	cl.Log.Printf("%s", logMsg)
	cl.ConsoleBroadcast(logMsg)
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

	logMsg := fmt.Sprintf("KICK %-20s [%d] %q\n", p.Name, p.ClientID, msg)
	cl.Log.Println(logMsg)
	cl.ConsoleBroadcast(logMsg)
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

// Setup a new cookie on a player
//
// Player cookies are a dirty and not terribly effective way of uniquely
// identifying players. Original Q2 made no effort to ID players other than
// their client number. Every player in the game can have the same name, skin,
// etc. The player's IP address was the only way to really differentiate them
// from other players. Now in the age of VPNs, an malicious player can get
// banned, and reconnect on a VPN with different IP, different name, etc, and
// continue being abusive.
//
// The idea of a player cookie is a persistent unique identifier. PlayerX can
// reconnect with a different name on a different IP with a different client
// and still be identified. This is great for tracking statistics and
// disciplinary actions (muting/banning shitheads).
func SetupPlayerCookie(cl *frontend.Frontend, p *frontend.Player) {
	value := hex.EncodeToString(crypto.RandomBytes(12)) // random ID

	// "modern" clients (q2pro, r1q2) support seta for archive vars
	a := fmt.Sprintf("seta cl_cookie %s", value)

	// ancient clients (3.2[01]) require old format "set name value a"
	//a_old := fmt.Sprintf("set cl_cookie %s a", value)

	u := "setu cl_cookie $cl_cookie"

	// tell player to write the var to local .cfg file for persistence
	StuffPlayer(cl, p, a)

	// tell player to add var to their userinfo string. This will
	// trigger a ClientUserinfoChanged() call on the game server
	StuffPlayer(cl, p, u)
}

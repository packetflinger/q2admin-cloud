// Website console: a server's live console output and sending commands to it.

package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/packetflinger/q2admind/frontend"
)

// ServerConsoleHandler shows the live console page for a frontend.
func ServerConsoleHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	data := PageResponse{SessionUser: user, Frontend: fe}
	fe.Mu.Lock()
	data.Head.Title = "Console: " + fe.Name + " | CloudAdmin"
	fe.Mu.Unlock()
	data.NavHighlight.Servers = "active"
	tmpl, err := parsePage("terminal.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "terminal", data, fe); err != nil {
		log.Println(err)
	}
}

// ConsoleStreamHandler streams a frontend's console output to the browser as
// server-sent events: the same feed SSH sessions watch.
func ConsoleStreamHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		http.Error(w, "401 - signed out", http.StatusUnauthorized)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the server's WriteTimeout would cut the stream off
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // tell nginx not to buffer this response

	stream := make(chan frontend.ConsoleLine, 64) // ConsoleBroadcast drops messages when this is full
	// a reconnecting browser sends the id of the last line it got, so it's
	// only sent what it missed; a newly opened console gets the whole backlog
	after, _ := strconv.ParseUint(r.Header.Get("Last-Event-ID"), 10, 64)
	backlog := fe.AddTerminal(&stream, after)
	defer fe.TerminalDisconnected(&stream)
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	statusCheck := time.NewTicker(consoleStatusEvery)
	defer statusCheck.Stop()

	send := func(line frontend.ConsoleLine) {
		data, _ := json.Marshal(line) // one line, whatever the text contains
		fmt.Fprintf(w, "id: %d\ndata: %s\n\n", line.Seq, data)
	}
	// the page header's map and player count, sent again whenever they change
	var lastStatus consoleStatus
	sendStatus := func() {
		if st := consoleStatusOf(fe); st != lastStatus {
			data, _ := json.Marshal(st)
			fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
			lastStatus = st
		}
	}
	fmt.Fprint(w, ": connected\n\n")
	sendStatus()
	for _, line := range backlog {
		send(line)
	}
	for {
		if err := rc.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
		case <-statusCheck.C:
			sendStatus()
		case line := <-stream:
			send(line)
		}
	}
}

// consoleStatusEvery is how often the console stream checks for a new map or
// player count.
var consoleStatusEvery = 5 * time.Second // a var so tests can shorten it

// consoleStatus is the console page header's view of its server.
type consoleStatus struct {
	Online  bool   `json:"online"`
	Map     string `json:"map"`
	Players int    `json:"players"`
	Max     int    `json:"max"`
}

// consoleStatusOf reads fe's map and player count. It locks fe briefly.
func consoleStatusOf(fe *frontend.Frontend) consoleStatus {
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	if !(fe.Connected && fe.Trusted) {
		return consoleStatus{}
	}
	return consoleStatus{Online: true, Map: fe.CurrentMap, Players: playersOnline(fe), Max: fe.MaxPlayers}
}

// validConsoleCommand checks a command typed by a user before it's sent to a
// server console. A newline would smuggle in extra commands.
func validConsoleCommand(cmd string) error {
	if cmd == "" || len(cmd) > 256 || strings.ContainsAny(cmd, "\r\n") {
		return errors.New("send one command of up to 256 characters")
	}
	return nil
}

// ConsoleCommandHandler runs one console command on a frontend, as if typed
// at its server console. Output arrives through ConsoleStreamHandler.
func ConsoleCommandHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		http.Error(w, "401 - signed out", http.StatusUnauthorized)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	cmd := LowerCommandName(strings.TrimSpace(r.PostFormValue("cmd"))) // so the echo shows what runs
	if err := validConsoleCommand(cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	if !(fe.Connected && fe.Trusted) {
		http.Error(w, "the server is offline", http.StatusConflict)
		return
	}
	fe.ConsoleBroadcast(fmt.Sprintf("[%s] > %s", DisplayName(user), cmd)) // other watchers see it too, before its output
	ConsoleCommand(fe, cmd)
	SendMessages(fe)
	fe.Log.Printf("web console [%s]: %s\n", user.GetEmail(), cmd)
	w.WriteHeader(http.StatusNoContent)
}

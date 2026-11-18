// SSH terminal output: printing to a session, its prompt, pausing the
// console stream and relaying a server's console to it.

package backend

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/packetflinger/q2admind/frontend"
)

const (
	TermMsgTypeGeneral = iota
	TermMsgTypePlayerChat
	TermMsgTypeBan
	TermMsgTypeMute
)

// TerminalMessage is a struct that is sent from the client to the SSH server
// to display. This allows for coloring certain information, etc.
type TerminalMessage struct {
	Type       int
	PlayerName string
	PlayerID   int
	Message    string
}

// linkFrontendToTerminal connects the frontend to the the ssh terminal to
// receive and display console messages. This is one-way from frontend to
// terminal to show things like connects, disconnects, chats, and internal
// q2admin stuff.
//
// The ssh user can select which client to watch. This is run concurrently
// and is stopped when the user switches frontends.
//
// The context arg is a "withCancel" context, so the calling func can terminate
// this go routine even when it's blocking waiting for input if needed.
func linkFrontendToTerminal(ctx context.Context, fe *frontend.Frontend, t *SSHTerminal, stream *chan frontend.ConsoleLine, backlog []frontend.ConsoleLine) {
	if fe == nil || t == nil {
		return
	}
	t.Println(yellow(fmt.Sprintf("* connecting to %s's console stream *", fe.Name)))

	show := func(line frontend.ConsoleLine) {
		msg := fmt.Sprintf("%s %s\n", time.Unix(line.Time, 0).Format("15:04:05"), line.Text)
		t.mu.Lock()
		paused := t.paused
		if paused > 0 && time.Now().Unix() < paused {
			t.buffer = append(t.buffer, msg)
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()
		if paused > 0 { // pause expired
			t.unpause()
		}
		t.Println(msg)
	}
	for _, line := range backlog { // recent history first
		show(line)
	}
	for {
		select {
		case line := <-*stream:
			show(line)
		case <-ctx.Done(): // cancel() called from SSH thread
			fe.TerminalDisconnected(stream)
			return
		}
	}
}

// unpause resumes the console stream, printing anything buffered while paused.
func (t *SSHTerminal) unpause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.paused == 0 {
		return
	}
	t.paused = 0
	for _, line := range t.buffer {
		t.Println(line)
	}
	t.buffer = nil
	t.RestorePrompt()
}

// Println will send str to the SSH terminal. If the input string is missing
// a newline, it's added before sending.
func (t *SSHTerminal) Println(str string) {
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
func (t *SSHTerminal) Printf(format string, a ...any) {
	if format == "" {
		return
	}
	str := fmt.Sprintf(format, a...)
	t.terminal.Write([]byte(str))
}

// Convert a logical variable (1/0, true/false, "yes"/"no") into an emoji
// checkmark
func checkMark(in any) string {
	mark := "\u2713"
	if reflect.TypeOf(in) == reflect.TypeFor[bool]() {
		if in == true {
			return mark
		}
	}
	if reflect.TypeOf(in) == reflect.TypeFor[int]() {
		if in == 1 {
			return mark
		}
	}
	if reflect.TypeOf(in) == reflect.TypeFor[string]() {
		if in == "yes" {
			return mark
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

// Helper func for using in templates
func connectionIndicator(f *frontend.Frontend) string {
	if f == nil {
		return "error"
	}
	if f.Connected && f.Trusted {
		return green("connected")
	}
	return red("offline  ") // pad with 2 space to match length
}

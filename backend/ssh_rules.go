// SSH rules command: listing, showing, deleting and adding a server's rules.

package backend

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/packetflinger/q2admind/frontend"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

// cmdRules lists the rules affecting the active server, or shows, deletes or
// adds one: "rules show <id>", "rules del <id>", "rules add".
func (ss *sshSession) cmdRules(c CmdArgs) {
	if c.argc == 0 {
		ss.term.Printf("%s %s\n", underline("Local rules affecting"), yellow(ss.active.Name))
		var msg bytes.Buffer
		if err := rulesTmpl.Execute(&msg, ss.active.Rules); err != nil {
			log.Println("error executing rules template:", err)
		}
		ss.term.Println(msg.String())
		msg.Reset()
		ss.term.Println(underline("Global rules:"))
		if err := rulesTmpl.Execute(&msg, be.rules); err != nil {
			log.Println("error executing rules template:", err)
		}
		ss.term.Println(msg.String())
	} else if c.argc > 1 && c.argv[0] == "show" {
		for _, r := range append(ss.active.Rules, be.rules...) {
			if strings.HasPrefix(r.GetUuid(), c.argv[1]) {
				ss.term.Printf(underline("Details for rule [%s]:\n\n"), yellow(c.argv[1]))
				ss.term.Println(prototext.Format(r))
				break
			}
		}
	} else if c.argc > 1 && (c.argv[0] == "del" || c.argv[0] == "delete" || c.argv[0] == "remove") {
		id := c.argv[1]
		if len(id) < 4 {
			ss.term.Printf("ID %q has too few characters\n", id)
			return
		}
		for _, r := range be.rules {
			if strings.HasPrefix(r.Uuid, id) {
				ss.term.Printf("Rule %q is applied globally, you can't remove it\n", r.Uuid)
				return
			}
		}
		var found *pb.Rule
		var newrules []*pb.Rule
		for i, r := range ss.active.Rules {
			if strings.HasPrefix(r.Uuid, id) {
				found = r
				continue
			}
			newrules = append(newrules, ss.active.Rules[i])
		}
		if found == nil {
			ss.term.Printf("error: no client-scoped rule found with ID %q\n", id)
			return
		}
		if err := ss.active.DeleteRule(found.Uuid); err != nil {
			log.Println(err)
			ss.term.Println("error removing the rule from the database")
			return
		}
		ss.active.Rules = newrules
		ss.term.Printf("Rule %q removed.\n", found.Uuid)
	} else if c.argc == 1 && c.argv[0] == "add" {
		ss.active.Mu.Unlock() // the wizard waits for input
		r, err := AddRuleWizard(ss.term, ss.active)
		ss.active.Mu.Lock()
		if err != nil {
			ss.term.Println(err.Error())
			return
		}
		ss.term.Printf("Adding rule proto:\n%s\n", prototext.Format(r))
		if err := ss.active.InsertRule(r); err != nil {
			log.Println(err)
			ss.term.Println("error saving the rule to the database")
			return
		}
		ss.active.Rules = append(ss.active.Rules, r)
	}
}

// AddRuleWizard will prompt the user to enter all the data needed to construct
// a rule proto affecting players.
func AddRuleWizard(t *SSHTerminal, fe *frontend.Frontend) (*pb.Rule, error) {
	if t == nil || fe == nil {
		return nil, fmt.Errorf("null terminal or frontend")
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

getaddr:
	t.Printf("  [Rule Wizard] What network address should this affect (CIDR notation (8.8.8.0/24))? ")
	in, err = t.terminal.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("error reading rule wizard input: %v", err)
	}
	// same parser the rule matcher uses
	if _, _, err := net.ParseCIDR(in); err != nil {
		t.Println("Invalid CIDR address")
		goto getaddr
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

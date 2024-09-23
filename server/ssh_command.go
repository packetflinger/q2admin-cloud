package server

import (
	"fmt"
	"strings"
)

const (
	SSHCmdQuit = iota
	SSHCmdServer
	SSHCmdSay
	SSHCmdWhois
	SSHCmdEmpty
	SSHCmdHelp
	SSHCmdStuff
)

type SSHCommand struct {
	cmd  int
	argc int      // how many args?
	argv []string // tokenized
	args string   // all in one string
}

func parseSSHCmd(input string) (SSHCommand, error) {
	if len(input) == 0 {
		return SSHCommand{cmd: SSHCmdEmpty}, nil
	}
	tokens := strings.Split(strings.ToLower(strings.Trim(input, " \n\t")), " ")
	c := tokens[0]
	if len(tokens) == 1 {
		switch c {
		case "server":
			return SSHCommand{cmd: SSHCmdServer}, nil
		case "?":
			fallthrough
		case "help":
			return SSHCommand{cmd: SSHCmdHelp}, nil
		case "quit":
			fallthrough
		case "exit":
			fallthrough
		case "logout":
			return SSHCommand{cmd: SSHCmdQuit}, nil
		}
	}
	if len(tokens) > 1 {
		command := SSHCommand{
			argc: len(tokens) - 1,
			argv: tokens[1:],
			args: strings.Join(tokens[1:], " "),
		}
		switch c {
		case "server":
			command.cmd = SSHCmdServer
			return command, nil
		case "say":
			command.cmd = SSHCmdSay
			return command, nil
		case "whois":
			command.cmd = SSHCmdWhois
			return command, nil
		case "stuff":
			command.cmd = SSHCmdStuff
			return command, nil
		}
	}
	return SSHCommand{}, fmt.Errorf("unknown command: %q", input)
}

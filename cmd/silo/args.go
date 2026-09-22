package main

import "fmt"

type invocation struct {
	command string
	args    []string
}

func parseArgs(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{command: "info"}, nil
	}

	command := args[0]

	switch command {
	case "list":
		if len(args) != 1 {
			return invocation{}, fmt.Errorf("too many arguments for 'list'")
		}
		return invocation{command: "list"}, nil
	case "create", "delete":
		if len(args) == 1 {
			return invocation{}, fmt.Errorf("missing name for %q", command)
		}
		if len(args) > 2 {
			return invocation{}, fmt.Errorf("too many arguments for %q", command)
		}
		return invocation{command: command, args: args[1:]}, nil
	case "ssh":
		if len(args) == 1 || args[1] == "--" {
			return invocation{}, fmt.Errorf("missing name for %q", command)
		}
		if len(args) > 2 && args[2] != "--" {
			return invocation{}, fmt.Errorf("SSH options must follow '--'")
		}

		sshArgs := []string{args[1]}
		if len(args) > 3 {
			sshArgs = append(sshArgs, args[3:]...)
		}
		return invocation{command: command, args: sshArgs}, nil
	case "start", "stop":
		if len(args) == 1 {
			return invocation{}, fmt.Errorf("missing name for %q", command)
		}
		if len(args) > 2 {
			return invocation{}, fmt.Errorf("too many arguments for %q", command)
		}
		return invocation{command: "update", args: args}, nil
	default:
		return invocation{}, fmt.Errorf("unknown command %q", command)
	}
}

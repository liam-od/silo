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
	case "create", "ssh":
		if len(args) == 1 {
			return invocation{}, fmt.Errorf("missing name for %q", command)
		}
		if len(args) > 2 {
			return invocation{}, fmt.Errorf("too many arguments for %q", command)
		}
		return invocation{command: command, args: args[1:]}, nil
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

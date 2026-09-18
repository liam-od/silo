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
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return invocation{}, fmt.Errorf("too many arguments for 'list'")
		}
		return invocation{command: "list"}, nil
	case "create":
		if len(args) == 1 {
			return invocation{}, fmt.Errorf("missing name for 'create'")
		}
		if len(args) > 2 {
			return invocation{}, fmt.Errorf("too many arguments for 'create'")
		}
		return invocation{command: "create", args: args[1:]}, nil
	default:
		return invocation{}, fmt.Errorf("unknown command %q", args[0])
	}
}

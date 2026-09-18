package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	incus "github.com/lxc/incus/v6/client"
)

func run(ctx context.Context, args []string) error {
	inv, err := parseArgs(args)
	if err != nil {
		return err
	}

	client, err := incus.ConnectIncusUnix("", nil)
	if err != nil {
		return fmt.Errorf("connect to Incus: %w", err)
	}

	switch inv.command {
	case "info":
		return showServerInfo(client)
	case "list":
		return listInstances(client)
	case "create":
		createCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()

		return createInstance(
			createCtx,
			client,
			inv.args[0],
			defaultImageSource(),
		)
	case "update":
		action := inv.args[0]
		name := inv.args[1]

		updateCtx, cancelUpdate := context.WithTimeout(ctx, 2*time.Minute)
		err := updateInstance(updateCtx, client, name, action)
		cancelUpdate()
		if err != nil {
			return err
		}
		if action != "start" {
			return nil
		}

		agentCtx, cancelAgent := context.WithTimeout(ctx, 2*time.Minute)
		err = waitForGuestAgent(agentCtx, client, name)
		cancelAgent()
		return err
	}
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "silo: %v\n", err)
		os.Exit(1)
	}
}

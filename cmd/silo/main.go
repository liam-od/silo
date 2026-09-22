package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"golang.org/x/term"
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
		config, err := getConfig()
		if err != nil {
			return err
		}

		if config.Instance.SSHPublicKeyPath == "" {
			return fmt.Errorf("instance.ssh_public_key is not configured")
		}

		publicKey, err := readSSHPublicKey(config.Instance.SSHPublicKeyPath)
		if err != nil {
			return fmt.Errorf(
				"read SSH public key %q: %w",
				config.Instance.SSHPublicKeyPath,
				err,
			)
		}

		createCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()

		return createInstance(
			createCtx,
			publicKey,
			client,
			inv.args[0],
			defaultImageSource(),
		)
	case "delete":
		confirm := interactiveDeletionConfirmer(
			os.Stdin,
			os.Stdout,
			term.IsTerminal(int(os.Stdin.Fd())),
		)
		return deleteInstance(ctx, client, inv.args[0], confirm, os.Stdout, removeKnownHost)
	case "update":
		action := inv.args[0]
		name := inv.args[1]

		updateCtx, cancelUpdate := context.WithTimeout(ctx, 2*time.Minute)
		err := updateInstance(updateCtx, client, os.Stdout, name, action, -1)
		cancelUpdate()
		if err != nil {
			return err
		}
		if action != "start" {
			return nil
		}

		return waitForInstanceReady(ctx, client, name)
	case "ssh":
		config, err := getConfig()
		if err != nil {
			return err
		}

		if config.Instance.SSHIdentityFile == "" {
			return fmt.Errorf("instance.ssh_identity_file is not configured")
		}

		return execSSH(client, inv.args[0], config.Instance.SSHIdentityFile, inv.args[1:])
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

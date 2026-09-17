package main

import (
	"fmt"
	"os"

	incus "github.com/lxc/incus/v6/client"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "silo: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	client, err := incus.ConnectIncusUnix("", nil)
	if err != nil {
		return fmt.Errorf("connect to Incus: %w", err)
	}
	server, _, err := client.GetServer()
	if err != nil {
		return fmt.Errorf("get Incus server information: %w", err)
	}

	fmt.Printf("Connected to Incus %s\n", server.Environment.ServerVersion)
	fmt.Printf("Project: %s\n", server.Environment.Project)

	return nil
}

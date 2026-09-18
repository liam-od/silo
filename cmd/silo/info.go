package main

import (
	"fmt"

	incus "github.com/lxc/incus/v6/client"
)

func showServerInfo(client incus.InstanceServer) error {
	server, _, err := client.GetServer()
	if err != nil {
		return fmt.Errorf("get Incus server information: %w", err)
	}

	fmt.Printf("Connected to Incus %s\n", server.Environment.ServerVersion)
	fmt.Printf("Project: %s\n", server.Environment.Project)

	return nil
}

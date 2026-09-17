package main

import (
	"fmt"
	"os"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "silo: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("too many arguments")
	}

	command := ""
	if len(args) == 1 {
		command = args[0]
	}

	if command != "" && command != "list" {
		return fmt.Errorf("unknown command %q", command)
	}

	client, err := incus.ConnectIncusUnix("", nil)
	if err != nil {
		return fmt.Errorf("connect to Incus: %w", err)
	}

	switch command {
	case "":
		server, _, err := client.GetServer()
		if err != nil {
			return fmt.Errorf("get Incus server information: %w", err)
		}

		fmt.Printf("Connected to Incus %s\n", server.Environment.ServerVersion)
		fmt.Printf("Project: %s\n", server.Environment.Project)
	case "list":
		instances, err := client.GetInstances(api.InstanceTypeVM)
		if err != nil {
			return fmt.Errorf("get VM instances: %w", err)
		}
		found := false
		for _, instance := range instances {
			if instance.Config["user.silo.managed"] == "true" {
				fmt.Printf("%s: %s\n", instance.Name, instance.Status)
				found = true
			}
		}
		if !found {
			fmt.Println("No Silo-managed VMs.")
		}
	}
	return nil
}

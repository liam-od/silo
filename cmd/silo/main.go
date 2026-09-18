package main

import (
	"fmt"
	"os"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type invocation struct {
	command string
	args    []string
}

type imageSource struct {
	reference string
	alias     string
	server    string
	protocol  string
}

type createRequestParams struct {
	name      string
	createdAt time.Time
	image     imageSource
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

func showServerInfo(client incus.InstanceServer) error {
	server, _, err := client.GetServer()
	if err != nil {
		return fmt.Errorf("get Incus server information: %w", err)
	}

	fmt.Printf("Connected to Incus %s\n", server.Environment.ServerVersion)
	fmt.Printf("Project: %s\n", server.Environment.Project)

	return nil
}

func listInstances(client incus.InstanceServer) error {
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

	return nil
}

func newCreateRequest(params createRequestParams) api.InstancesPost {
	return api.InstancesPost{
		InstancePut: api.InstancePut{
			Config: map[string]string{
				"user.silo.managed":    "true",
				"user.silo.image":      params.image.reference,
				"user.silo.created-at": params.createdAt.UTC().Format(time.RFC3339),
			},
			Profiles: []string{"default"},
		},
		Name: params.name,
		Source: api.InstanceSource{
			Type:     "image",
			Mode:     "pull",
			Server:   params.image.server,
			Protocol: params.image.protocol,
			Alias:    params.image.alias,
		},
		Type:  api.InstanceTypeVM,
		Start: false,
	}
}

func run(args []string) error {
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
		return fmt.Errorf("create is not implemented")
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "silo: %v\n", err)
		os.Exit(1)
	}
}

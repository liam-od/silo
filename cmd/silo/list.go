package main

import (
	"fmt"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

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

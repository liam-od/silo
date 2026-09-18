package main

import (
	"context"
	"fmt"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

func updateInstance(
	ctx context.Context,
	client incus.InstanceServer,
	name, action string,
) error {
	fmt.Printf("Updating %s state: %s...\n", name, action)

	op, err := client.UpdateInstanceState(name, api.InstanceStatePut{
		Action:  action,
		Timeout: -1,
	}, "")

	if err != nil {
		return fmt.Errorf("submit %s for instance %q: %w", action, name, err)
	}

	err = op.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for instance %q to %s: %w", name, action, err)
	}

	fmt.Printf("Updated %s state: %s.\n", name, action)
	return nil
}

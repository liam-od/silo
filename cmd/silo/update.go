package main

import (
	"context"
	"fmt"
	"io"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type instanceStateUpdater interface {
	UpdateInstanceState(name string, state api.InstanceStatePut, etag string) (incus.Operation, error)
}

func updateInstance(
	ctx context.Context,
	client instanceStateUpdater,
	output io.Writer,
	name, action string,
	serverTimeout int,
) error {
	fmt.Fprintf(output, "Updating %s state: %s...\n", name, action)

	op, err := client.UpdateInstanceState(name, api.InstanceStatePut{
		Action:  action,
		Timeout: serverTimeout,
	}, "")

	if err != nil {
		return fmt.Errorf("submit %s for instance %q: %w", action, name, err)
	}

	err = op.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for instance %q to %s: %w", name, action, err)
	}

	fmt.Fprintf(output, "Updated %s state: %s.\n", name, action)
	return nil
}

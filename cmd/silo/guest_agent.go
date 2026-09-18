package main

import (
	"context"
	"fmt"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

func waitForGuestAgent(
	ctx context.Context,
	client incus.InstanceServer,
	name string,
) error {
	fmt.Printf("Waiting for guest agent on %s...\n", name)

	request := api.InstanceExecPost{
		Command: []string{"/bin/true"},
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		op, err := client.ExecInstance(name, request, nil)
		if err == nil {
			if err := op.WaitContext(ctx); err == nil {
				fmt.Printf("Guest agent ready on %s.\n", name)
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for guest agent on %q: %w", name, ctx.Err())
		case <-ticker.C:
		}
	}
}

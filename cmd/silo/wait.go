package main

import (
	"context"
	"fmt"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

func waitForInstanceReady(
	ctx context.Context,
	client incus.InstanceServer,
	name string,
) error {
	agentCtx, cancelAgent := context.WithTimeout(ctx, 2*time.Minute)
	err := waitForGuestAgent(agentCtx, client, name)
	cancelAgent()
	if err != nil {
		return err
	}

	cloudInitCtx, cancelCloudInit := context.WithTimeout(ctx, 5*time.Minute)
	err = waitForCloudInit(cloudInitCtx, client, name)
	cancelCloudInit()
	return err
}

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

func waitForCloudInit(
	ctx context.Context,
	client incus.InstanceServer,
	name string,
) error {
	fmt.Printf("Waiting for cloud-init on %s...\n", name)

	request := api.InstanceExecPost{
		Command: []string{"/usr/bin/cloud-init", "status", "--wait"},
	}
	op, err := client.ExecInstance(name, request, nil)
	if err != nil {
		return fmt.Errorf("submit cloud-init status on %q: %w", name, err)
	}

	err = op.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for cloud-init on %q: %w", name, err)
	}

	fmt.Printf("Cloud-init complete on %s.\n", name)
	return nil
}

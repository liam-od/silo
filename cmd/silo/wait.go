package main

import (
	"context"
	"fmt"
	"net"
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
	if err != nil {
		return err
	}

	guestAddressCtx, cancelGuestAddress := context.WithTimeout(ctx, 2*time.Minute)
	_, err = waitForGuestAddress(guestAddressCtx, client, name)
	cancelGuestAddress()

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

func waitForGuestAddress(ctx context.Context, client incus.InstanceServer, name string) (string, error) {
	fmt.Printf("Waiting for address on %s...\n", name)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		state, _, err := client.GetInstanceState(name)
		if err != nil {
			return "", fmt.Errorf("get instance state for %q: %w", name, err)
		}

		address, found, err := findGuestIPv4(state.Network)
		if err != nil {
			return "", fmt.Errorf("select guest address for %q: %w", name, err)
		}
		if found {
			fmt.Printf("Address ready on %s: %s\n", name, address)
			return address, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("wait for guest address on %q: %w", name, ctx.Err())
		case <-ticker.C:
		}
	}
}

func findGuestIPv4(networks map[string]api.InstanceStateNetwork) (string, bool, error) {
	var found string
	for _, network := range networks {
		for _, address := range network.Addresses {
			if address.Family != "inet" ||
				address.Scope != "global" ||
				net.ParseIP(address.Address).To4() == nil {
				continue
			}

			if found != "" && found != address.Address {
				return "", false, fmt.Errorf("multiple global IPv4 addresses found")
			}
			found = address.Address
		}
	}

	return found, found != "", nil
}

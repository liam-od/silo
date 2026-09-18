package main

import (
	"context"
	"fmt"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type imageSource struct {
	reference string
	alias     string
	server    string
	protocol  string
}

type createInstanceRequestParams struct {
	name      string
	createdAt time.Time
	image     imageSource
}

func defaultImageSource() imageSource {
	return imageSource{
		reference: "images:ubuntu/24.04/cloud",
		alias:     "ubuntu/24.04/cloud",
		server:    "https://images.linuxcontainers.org",
		protocol:  "simplestreams",
	}
}

func newCreateRequest(params createInstanceRequestParams) api.InstancesPost {
	return api.InstancesPost{
		Config: map[string]string{
			"user.silo.managed":    "true",
			"user.silo.image":      params.image.reference,
			"user.silo.created-at": params.createdAt.UTC().Format(time.RFC3339),
		},
		Profiles: []string{"default"},
		Name:     params.name,
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

func createInstance(
	ctx context.Context,
	client incus.InstanceServer,
	name string,
	image imageSource,
) error {
	params := createInstanceRequestParams{
		name:      name,
		createdAt: time.Now(),
		image:     image,
	}
	request := newCreateRequest(params)
	fmt.Printf("Creating %s from %s...\n", name, image.alias)

	op, err := client.CreateInstance(request)
	if err != nil {
		return fmt.Errorf("submit create instance: %w", err)
	}
	err = op.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("create instance operation: %w", err)
	}

	fmt.Printf("Created %s (stopped).\n", name)

	startCtx, cancelStart := context.WithTimeout(ctx, 2*time.Minute)
	err = updateInstance(startCtx, client, name, "start")
	cancelStart()
	if err != nil {
		return err
	}

	agentCtx, cancelAgent := context.WithTimeout(ctx, 2*time.Minute)
	err = waitForGuestAgent(agentCtx, client, name)
	cancelAgent()
	return err
}

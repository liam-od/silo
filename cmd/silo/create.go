package main

import (
	"context"
	"fmt"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"gopkg.in/yaml.v3"
)

const defaultInstanceUser = "agent"

type imageSource struct {
	reference string
	alias     string
}

type createInstanceRequestParams struct {
	name      string
	createdAt time.Time
	image     imageSource
	userData  string
}

type cloudInitConfig struct {
	Hostname       string          `yaml:"hostname"`
	ManageEtcHosts bool            `yaml:"manage_etc_hosts"`
	Users          []cloudInitUser `yaml:"users"`
	SSHPwauth      bool            `yaml:"ssh_pwauth"`
}

type cloudInitUser struct {
	Name              string   `yaml:"name"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys"`
}

func defaultImageSource() imageSource {
	return imageSource{
		reference: "silo-dev-v1",
		alias:     "silo-dev-v1",
	}
}

func newCreateRequest(params createInstanceRequestParams) api.InstancesPost {
	return api.InstancesPost{
		Config: map[string]string{
			"user.silo.managed":    "true",
			"user.silo.image":      params.image.reference,
			"user.silo.created-at": params.createdAt.UTC().Format(time.RFC3339),
			"cloud-init.user-data": params.userData,
		},
		Profiles: []string{"default"},
		Name:     params.name,
		Source: api.InstanceSource{
			Type:  "image",
			Alias: params.image.alias,
		},
		Type:  api.InstanceTypeVM,
		Start: false,
	}
}

func newCloudInitUserData(instanceName, publicKey string) (string, error) {
	if instanceName == "" {
		return "", fmt.Errorf("cloud-init hostname is empty")
	}
	if publicKey == "" {
		return "", fmt.Errorf("cloud-init SSH public key is empty")
	}

	config := cloudInitConfig{
		Hostname:       instanceName,
		ManageEtcHosts: true,
		SSHPwauth:      false,
		Users: []cloudInitUser{
			{
				Name:              defaultInstanceUser,
				SSHAuthorizedKeys: []string{publicKey},
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode cloud-init user data: %w", err)
	}

	return "#cloud-config\n" + string(data), nil
}

func createInstance(
	ctx context.Context,
	publicKey string,
	client incus.InstanceServer,
	name string,
	image imageSource,
) error {
	userData, err := newCloudInitUserData(name, publicKey)
	if err != nil {
		return err
	}

	params := createInstanceRequestParams{
		name:      name,
		createdAt: time.Now(),
		image:     image,
		userData:  userData,
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

	if err := waitForInstanceReady(ctx, client, name); err != nil {
		return err
	}

	return nil
}

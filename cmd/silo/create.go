package main

import (
	"context"
	"fmt"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"gopkg.in/yaml.v3"
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
	Shell             string   `yaml:"shell"`
	Groups            []string `yaml:"groups"`
	Sudo              []string `yaml:"sudo"`
	LockPasswd        bool     `yaml:"lock_passwd"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys"`
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
			"cloud-init.user-data": params.userData,
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

func newCloudInitUserData(instanceName, user, publicKey string) (string, error) {
	if instanceName == "" {
		return "", fmt.Errorf("cloud-init hostname is empty")
	}
	if user == "" {
		return "", fmt.Errorf("cloud-init user is empty")
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
				Name:              user,
				Shell:             "/bin/bash",
				Groups:            []string{"sudo"},
				Sudo:              []string{"ALL=(ALL) NOPASSWD:ALL"},
				LockPasswd:        true,
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
	config siloConfig,
	client incus.InstanceServer,
	name string,
	image imageSource,
) error {
	userData, err := newCloudInitUserData(name, config.Instance.SSHUser, config.Instance.SSHPublicKey)
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

	err = waitForInstanceReady(ctx, client, name)
	if err != nil {
		return err
	}

	return nil
}

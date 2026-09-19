package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lxc/incus/v6/shared/api"
	"gopkg.in/yaml.v3"
)

func TestNewCreateRequest(t *testing.T) {
	createdAt := time.Date(
		2026, time.September, 17,
		14, 30, 0, 0,
		time.FixedZone("UTC+2", 2*60*60),
	)
	image := imageSource{
		reference: "test:silo-dev-v1",
		alias:     "silo-dev-v1",
		server:    "https://example.invalid",
		protocol:  "simplestreams",
	}

	const userData = "#cloud-config\nhostname: test-1\n"
	got := newCreateRequest(createInstanceRequestParams{
		name:      "test-1",
		createdAt: createdAt,
		image:     image,
		userData:  userData,
	})

	if got.Name != "test-1" {
		t.Errorf("name = %q, want %q", got.Name, "test-1")
	}
	if got.Type != api.InstanceTypeVM {
		t.Errorf("type = %q, want %q", got.Type, api.InstanceTypeVM)
	}
	if got.Start {
		t.Error("start = true, want false")
	}
	if !slices.Equal(got.Profiles, []string{"default"}) {
		t.Errorf("profiles = %v, want [default]", got.Profiles)
	}

	if got.Source.Type != "image" {
		t.Errorf("source type = %q, want %q", got.Source.Type, "image")
	}
	if got.Source.Mode != "pull" {
		t.Errorf("source mode = %q, want %q", got.Source.Mode, "pull")
	}
	if got.Source.Server != image.server {
		t.Errorf("source server = %q, want %q", got.Source.Server, image.server)
	}
	if got.Source.Protocol != image.protocol {
		t.Errorf("source protocol = %q, want %q", got.Source.Protocol, image.protocol)
	}
	if got.Source.Alias != image.alias {
		t.Errorf("source alias = %q, want %q", got.Source.Alias, image.alias)
	}

	if got.Config["user.silo.managed"] != "true" {
		t.Errorf("managed metadata = %q, want %q", got.Config["user.silo.managed"], "true")
	}
	if got.Config["user.silo.image"] != image.reference {
		t.Errorf("image metadata = %q, want %q", got.Config["user.silo.image"], image.reference)
	}
	if got.Config["cloud-init.user-data"] != userData {
		t.Errorf("cloud-init user data = %q, want %q", got.Config["cloud-init.user-data"], userData)
	}

	const wantCreatedAt = "2026-09-17T12:30:00Z"
	if got.Config["user.silo.created-at"] != wantCreatedAt {
		t.Errorf(
			"created-at metadata = %q, want %q",
			got.Config["user.silo.created-at"],
			wantCreatedAt,
		)
	}
}

func TestNewCloudInitUserData(t *testing.T) {
	const publicKey = "ssh-ed25519 key-material developer: laptop #1"
	got, err := newCloudInitUserData("test-1", "agent", publicKey)
	if err != nil {
		t.Fatalf("newCloudInitUserData() error = %v", err)
	}

	const header = "#cloud-config\n"
	if !strings.HasPrefix(got, header) {
		t.Fatalf("user data does not start with %q: %q", header, got)
	}

	var config cloudInitConfig
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(got, header)), &config); err != nil {
		t.Fatalf("decode generated cloud-init data: %v", err)
	}

	if config.Hostname != "test-1" {
		t.Errorf("hostname = %q, want %q", config.Hostname, "test-1")
	}
	if !config.ManageEtcHosts {
		t.Error("manage_etc_hosts = false, want true")
	}
	if config.SSHPwauth {
		t.Error("ssh_pwauth = true, want false")
	}
	if len(config.Users) != 1 {
		t.Fatalf("users count = %d, want 1", len(config.Users))
	}

	user := config.Users[0]
	if user.Name != "agent" {
		t.Errorf("user name = %q, want %q", user.Name, "agent")
	}
	if user.Shell != "/bin/bash" {
		t.Errorf("shell = %q, want %q", user.Shell, "/bin/bash")
	}
	if !slices.Equal(user.Groups, []string{"sudo"}) {
		t.Errorf("groups = %v, want [sudo]", user.Groups)
	}
	if !slices.Equal(user.Sudo, []string{"ALL=(ALL) NOPASSWD:ALL"}) {
		t.Errorf("sudo = %v, want [ALL=(ALL) NOPASSWD:ALL]", user.Sudo)
	}
	if !user.LockPasswd {
		t.Error("lock_passwd = false, want true")
	}
	if !slices.Equal(user.SSHAuthorizedKeys, []string{publicKey}) {
		t.Errorf("SSH authorized keys = %v, want [%q]", user.SSHAuthorizedKeys, publicKey)
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(got, header)), &raw); err != nil {
		t.Fatalf("decode generated cloud-init data as map: %v", err)
	}
	if value, ok := raw["ssh_pwauth"]; !ok || value != false {
		t.Errorf("ssh_pwauth field = %v, present = %v; want false and present", value, ok)
	}
}

func TestNewCloudInitUserDataRejectsEmptyRequiredValues(t *testing.T) {
	tests := []struct {
		name      string
		instance  string
		user      string
		publicKey string
	}{
		{name: "empty hostname", user: "agent", publicKey: "ssh-ed25519 key-material"},
		{name: "empty user", instance: "test-1", publicKey: "ssh-ed25519 key-material"},
		{name: "empty public key", instance: "test-1", user: "agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newCloudInitUserData(tt.instance, tt.user, tt.publicKey); err == nil {
				t.Fatal("newCloudInitUserData() error = nil, want an error")
			}
		})
	}
}

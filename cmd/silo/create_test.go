package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lxc/incus/v6/shared/api"
	"gopkg.in/yaml.v3"
)

func TestDefaultImageSource(t *testing.T) {
	got := defaultImageSource()
	if got.reference != "silo-dev-v1" {
		t.Errorf("reference = %q, want %q", got.reference, "silo-dev-v1")
	}
	if got.alias != "silo-dev-v1" {
		t.Errorf("alias = %q, want %q", got.alias, "silo-dev-v1")
	}
}

func TestNewCreateRequest(t *testing.T) {
	createdAt := time.Date(
		2026, time.September, 17,
		14, 30, 0, 0,
		time.FixedZone("UTC+2", 2*60*60),
	)
	image := imageSource{
		reference: "silo-dev-v1",
		alias:     "silo-dev-v1",
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
	if got.Source.Mode != "" {
		t.Errorf("source mode = %q, want empty for local image", got.Source.Mode)
	}
	if got.Source.Server != "" {
		t.Errorf("source server = %q, want empty for local image", got.Source.Server)
	}
	if got.Source.Protocol != "" {
		t.Errorf("source protocol = %q, want empty for local image", got.Source.Protocol)
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
	got, err := newCloudInitUserData("test-1", publicKey)
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
	if !slices.Equal(user.SSHAuthorizedKeys, []string{publicKey}) {
		t.Errorf("SSH authorized keys = %v, want [%q]", user.SSHAuthorizedKeys, publicKey)
	}

	var raw struct {
		SSHPwauth any              `yaml:"ssh_pwauth"`
		Users     []map[string]any `yaml:"users"`
	}
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(got, header)), &raw); err != nil {
		t.Fatalf("decode generated cloud-init data as map: %v", err)
	}
	if raw.SSHPwauth != false {
		t.Errorf("ssh_pwauth field = %v, want false", raw.SSHPwauth)
	}
	for _, field := range []string{"shell", "groups", "sudo", "lock_passwd"} {
		if _, ok := raw.Users[0][field]; ok {
			t.Errorf("cloud-init user unexpectedly contains image-owned field %q", field)
		}
	}
}

func TestNewCloudInitUserDataRejectsEmptyRequiredValues(t *testing.T) {
	tests := []struct {
		name      string
		instance  string
		publicKey string
	}{
		{name: "empty hostname", publicKey: "ssh-ed25519 key-material"},
		{name: "empty public key", instance: "test-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newCloudInitUserData(tt.instance, tt.publicKey); err == nil {
				t.Fatal("newCloudInitUserData() error = nil, want an error")
			}
		})
	}
}

package main

import (
	"slices"
	"testing"
	"time"

	"github.com/lxc/incus/v6/shared/api"
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

	got := newCreateRequest(createInstanceRequestParams{
		name:      "test-1",
		createdAt: createdAt,
		image:     image,
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

	const wantCreatedAt = "2026-09-17T12:30:00Z"
	if got.Config["user.silo.created-at"] != wantCreatedAt {
		t.Errorf(
			"created-at metadata = %q, want %q",
			got.Config["user.silo.created-at"],
			wantCreatedAt,
		)
	}
}

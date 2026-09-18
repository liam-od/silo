package main

import (
	"slices"
	"testing"
	"time"

	"github.com/lxc/incus/v6/shared/api"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    invocation
		wantErr bool
	}{
		{
			name:  "no arguments selects info",
			input: nil,
			want:  invocation{command: "info"},
		},
		{
			name:  "list command",
			input: []string{"list"},
			want:  invocation{command: "list"},
		},
		{
			name:  "create command with name",
			input: []string{"create", "test-1"},
			want: invocation{
				command: "create",
				args:    []string{"test-1"},
			},
		},
		{
			name:    "create without name",
			input:   []string{"create"},
			wantErr: true,
		},
		{
			name:    "create with extra argument",
			input:   []string{"create", "test-1", "extra"},
			wantErr: true,
		},
		{
			name:    "list with extra argument",
			input:   []string{"list", "extra"},
			wantErr: true,
		},
		{
			name:    "unknown command",
			input:   []string{"unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if got.command != tt.want.command {
				t.Errorf("command = %q, want %q", got.command, tt.want.command)
			}

			if !slices.Equal(got.args, tt.want.args) {
				t.Errorf("args = %v, want %v", got.args, tt.want.args)
			}
		})
	}
}

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

	got := newCreateRequest(createRequestParams{
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

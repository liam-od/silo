package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/lxc/incus/v6/shared/api"
)

func TestWaitForPort(t *testing.T) {
	t.Run("listening port is ready", func(t *testing.T) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer listener.Close()

		port := listener.Addr().(*net.TCPAddr).Port
		if err := waitForPort(t.Context(), "127.0.0.1", fmt.Sprint(port)); err != nil {
			t.Fatalf("waitForPort() error = %v", err)
		}
	})

	t.Run("context deadline is returned", func(t *testing.T) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		if err := listener.Close(); err != nil {
			t.Fatalf("close listener: %v", err)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()

		err = waitForPort(ctx, "127.0.0.1", fmt.Sprint(port))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waitForPort() error = %v, want context deadline exceeded", err)
		}
	})
}

func TestFindGuestIPv4(t *testing.T) {
	tests := []struct {
		name      string
		networks  map[string]api.InstanceStateNetwork
		want      string
		wantFound bool
		wantErr   bool
	}{
		{
			name: "one global IPv4",
			networks: map[string]api.InstanceStateNetwork{
				"enp5s0": {
					Addresses: []api.InstanceStateNetworkAddress{
						{Family: "inet", Address: "10.24.55.63", Scope: "global"},
						{Family: "inet6", Address: "fe80::1", Scope: "link"},
					},
				},
				"lo": {
					Addresses: []api.InstanceStateNetworkAddress{
						{Family: "inet", Address: "127.0.0.1", Scope: "local"},
					},
				},
			},
			want:      "10.24.55.63",
			wantFound: true,
		},
		{
			name: "loopback only",
			networks: map[string]api.InstanceStateNetwork{
				"lo": {
					Addresses: []api.InstanceStateNetworkAddress{
						{Family: "inet", Address: "127.0.0.1", Scope: "local"},
					},
				},
			},
		},
		{
			name: "IPv6 only",
			networks: map[string]api.InstanceStateNetwork{
				"enp5s0": {
					Addresses: []api.InstanceStateNetworkAddress{
						{Family: "inet6", Address: "fe80::1", Scope: "link"},
					},
				},
			},
		},
		{
			name:     "no addresses",
			networks: map[string]api.InstanceStateNetwork{},
		},
		{
			name: "multiple global IPv4 addresses",
			networks: map[string]api.InstanceStateNetwork{
				"enp5s0": {
					Addresses: []api.InstanceStateNetworkAddress{
						{Family: "inet", Address: "10.24.55.63", Scope: "global"},
					},
				},
				"enp6s0": {
					Addresses: []api.InstanceStateNetworkAddress{
						{Family: "inet", Address: "192.0.2.10", Scope: "global"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := findGuestIPv4(tt.networks)
			if (err != nil) != tt.wantErr {
				t.Fatalf("findGuestIPv4() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if got != tt.want {
				t.Errorf("address = %q, want %q", got, tt.want)
			}
		})
	}
}

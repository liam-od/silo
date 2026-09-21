package main

import (
	"slices"
	"testing"
)

func TestBuildSSHArgs(t *testing.T) {
	baseInvocation := sshInvocation{
		address:        "10.24.55.104",
		instanceID:     "b6039210-814c-46a1-a062-3dd28c7f136e",
		identityFile:   "/home/liam/.ssh/silo",
		knownHostsFile: "/home/liam/.config/silo/known_hosts",
	}
	fixedArgs := []string{
		"-i", "/home/liam/.ssh/silo",
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=/home/liam/.config/silo/known_hosts",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "HostKeyAlias=silo-b6039210-814c-46a1-a062-3dd28c7f136e",
		"-o", "CheckHostIP=no",
		"-o", "LogLevel=ERROR",
	}

	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{
			name: "basic SSH",
			want: append(
				slices.Clone(fixedArgs),
				"agent@10.24.55.104",
			),
		},
		{
			name:    "forward agent and port",
			options: []string{"-A", "-L", "3000:localhost:3000"},
			want: append(
				slices.Clone(fixedArgs),
				"-A", "-L", "3000:localhost:3000",
				"agent@10.24.55.104",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invocation := baseInvocation
			invocation.options = tt.options

			got := buildSSHArgs(invocation)
			if !slices.Equal(got, tt.want) {
				t.Errorf("arguments = %v, want %v", got, tt.want)
			}
		})
	}
}

package main

import (
	"slices"
	"testing"
)

func TestBuildSSHArgs(t *testing.T) {
	connection := sshConnection{
		address:        "10.24.55.104",
		instanceID:     "b6039210-814c-46a1-a062-3dd28c7f136e",
		identityFile:   "/home/liam/.ssh/silo",
		knownHostsFile: "/home/liam/.config/silo/known_hosts",
	}

	got := buildSSHArgs(connection)
	want := []string{
		"-i", "/home/liam/.ssh/silo",
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=/home/liam/.config/silo/known_hosts",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "HostKeyAlias=silo-b6039210-814c-46a1-a062-3dd28c7f136e",
		"-o", "CheckHostIP=no",
		"-o", "LogLevel=ERROR",
		"agent@10.24.55.104",
	}

	if !slices.Equal(got, want) {
		t.Errorf("arguments = %v, want %v", got, want)
	}
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	incus "github.com/lxc/incus/v6/client"
)

type sshInvocation struct {
	address        string
	instanceID     string
	identityFile   string
	knownHostsFile string
	options        []string
}

func buildSSHArgs(inv sshInvocation) []string {
	args := []string{
		"-i", inv.identityFile,
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=" + inv.knownHostsFile,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "HostKeyAlias=silo-" + inv.instanceID,
		"-o", "CheckHostIP=no",
		"-o", "LogLevel=ERROR",
	}
	args = append(args, inv.options...)
	return append(args, defaultInstanceUser+"@"+inv.address)
}

func getInstanceID(client incus.InstanceServer, name string) (string, error) {
	instance, _, err := client.GetInstance(name)
	if err != nil {
		return "", fmt.Errorf("get instance %q: %w", name, err)
	}

	instanceID := instance.Config["volatile.uuid"]
	if instanceID == "" {
		return "", fmt.Errorf("instance %q has no UUID", name)
	}

	return instanceID, nil
}

func execSSH(client incus.InstanceServer, name, identityFile string, options []string) error {
	address, found, err := getGuestAddress(client, name)
	if err != nil {
		return fmt.Errorf("get guest address: %w", err)
	}
	if !found {
		return fmt.Errorf("instance %q has no global IPv4 address", name)
	}

	instanceID, err := getInstanceID(client, name)
	if err != nil {
		return fmt.Errorf("get instance id: %w", err)
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("get user config directory: %w", err)
	}

	siloDir := filepath.Join(configDir, "silo")
	if err := os.MkdirAll(siloDir, 0o700); err != nil {
		return fmt.Errorf("create Silo config directory: %w", err)
	}

	inv := sshInvocation{
		address:        address,
		instanceID:     instanceID,
		identityFile:   identityFile,
		knownHostsFile: filepath.Join(siloDir, "known_hosts"),
		options:        options,
	}
	args := buildSSHArgs(inv)

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("find system SSH client: %w", err)
	}

	argv := append([]string{"ssh"}, args...)
	if err := syscall.Exec(sshPath, argv, os.Environ()); err != nil {
		return fmt.Errorf("execute SSH client: %w", err)
	}

	return nil
}

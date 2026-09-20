package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	incus "github.com/lxc/incus/v6/client"
)

type sshConnection struct {
	address        string
	instanceID     string
	identityFile   string
	knownHostsFile string
}

func buildSSHArgs(con sshConnection) []string {
	return []string{
		"-i", con.identityFile,
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=" + con.knownHostsFile,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "HostKeyAlias=silo-" + con.instanceID,
		"-o", "CheckHostIP=no",
		"-o", "LogLevel=ERROR",
		defaultInstanceUser + "@" + con.address,
	}
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

func execSSH(client incus.InstanceServer, name, identityFile string) error {
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

	con := sshConnection{
		address:        address,
		instanceID:     instanceID,
		identityFile:   identityFile,
		knownHostsFile: filepath.Join(siloDir, "known_hosts"),
	}
	args := buildSSHArgs(con)

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

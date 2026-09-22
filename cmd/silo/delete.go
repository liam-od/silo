package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

const (
	deleteStopGracePeriod   = 90 * time.Second
	deleteStopWaitTimeout   = 2 * time.Minute
	deleteOperationTimeout  = 2 * time.Minute
	knownHostCleanupTimeout = 10 * time.Second
)

type instanceDeleteClient interface {
	instanceStateUpdater
	GetInstance(name string) (*api.Instance, string, error)
	GetInstanceSnapshots(instanceName string) ([]api.InstanceSnapshot, error)
	GetInstanceBackups(instanceName string) ([]api.InstanceBackup, error)
	DeleteInstance(name string) (incus.Operation, error)
}

type deletionConfirmer func(name string) (bool, error)
type knownHostCleaner func(ctx context.Context, alias string) error
type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func acceptsDeletionConfirmation(response string) bool {
	switch strings.ToLower(strings.TrimSpace(response)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func confirmDeletion(input io.Reader, output io.Writer, name string) (bool, error) {
	if _, err := fmt.Fprintf(output, "Delete Silo-managed instance %q? [y/N] ", name); err != nil {
		return false, fmt.Errorf("write deletion confirmation: %w", err)
	}

	response, err := bufio.NewReader(input).ReadString('\n')
	if errors.Is(err, io.EOF) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read deletion confirmation: %w", err)
	}

	return acceptsDeletionConfirmation(response), nil
}

func interactiveDeletionConfirmer(input io.Reader, output io.Writer, interactive bool) deletionConfirmer {
	return func(name string) (bool, error) {
		if !interactive {
			return false, fmt.Errorf("deletion confirmation requires an interactive terminal")
		}
		return confirmDeletion(input, output, name)
	}
}

func knownHostAlias(instanceID string) string {
	return "silo-" + instanceID
}

func buildKnownHostCleanupArgs(alias, knownHostsFile string) []string {
	return []string{"-R", alias, "-f", knownHostsFile}
}

func runCleanupCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func removeKnownHostFromFile(
	ctx context.Context,
	alias, knownHostsFile, sshKeygen string,
	run commandRunner,
) error {
	if _, err := os.Stat(knownHostsFile); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect known-hosts file %q: %w", knownHostsFile, err)
	}

	output, err := run(ctx, sshKeygen, buildKnownHostCleanupArgs(alias, knownHostsFile)...)
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("remove %q from %q: %w: %s", alias, knownHostsFile, err, detail)
		}
		return fmt.Errorf("remove %q from %q: %w", alias, knownHostsFile, err)
	}
	return nil
}

func removeKnownHost(ctx context.Context, alias string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("get user config directory: %w", err)
	}
	knownHostsFile := filepath.Join(configDir, "silo", "known_hosts")
	if _, err := os.Stat(knownHostsFile); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect known-hosts file %q: %w", knownHostsFile, err)
	}

	sshKeygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return fmt.Errorf("find system ssh-keygen: %w", err)
	}
	return removeKnownHostFromFile(ctx, alias, knownHostsFile, sshKeygen, runCleanupCommand)
}

func fetchDeletionTarget(
	client instanceDeleteClient,
	name, expectedID string,
) (*api.Instance, error) {
	if strings.Contains(name, "/") {
		return nil, fmt.Errorf("%q is a snapshot or backup name; refusing to delete", name)
	}

	instance, _, err := client.GetInstance(name)
	if err != nil {
		return nil, fmt.Errorf("fetch instance %q before deletion: %w", name, err)
	}
	if instance.Name != name {
		return nil, fmt.Errorf("fetched instance name %q does not match requested name %q; refusing to delete", instance.Name, name)
	}
	if instance.Type != string(api.InstanceTypeVM) {
		return nil, fmt.Errorf("instance %q is not a VM; refusing to delete", name)
	}
	if instance.Config["user.silo.managed"] != "true" {
		return nil, fmt.Errorf("instance %q is not Silo-managed; refusing to delete", name)
	}

	instanceID := instance.Config["volatile.uuid"]
	if instanceID == "" {
		return nil, fmt.Errorf("instance %q has no UUID; refusing to delete", name)
	}
	if expectedID != "" && instanceID != expectedID {
		return nil, fmt.Errorf("instance %q was replaced while awaiting deletion confirmation; refusing to delete", name)
	}

	snapshots, err := client.GetInstanceSnapshots(name)
	if err != nil {
		return nil, fmt.Errorf("check instance %q for snapshots: %w", name, err)
	}
	if len(snapshots) != 0 {
		return nil, fmt.Errorf("instance %q has %d snapshot(s); refusing to delete", name, len(snapshots))
	}

	backups, err := client.GetInstanceBackups(name)
	if err != nil {
		return nil, fmt.Errorf("check instance %q for backups: %w", name, err)
	}
	if len(backups) != 0 {
		return nil, fmt.Errorf("instance %q has %d backup(s); refusing to delete", name, len(backups))
	}

	switch instance.StatusCode {
	case api.Running, api.Stopped:
		return instance, nil
	default:
		return nil, fmt.Errorf(
			"instance %q is %s; only running or stopped VMs can be safely deleted",
			name,
			instance.StatusCode,
		)
	}
}

func deletionCompletedAfterWaitError(
	client instanceDeleteClient,
	name, instanceID string,
	waitErr error,
) (bool, error) {
	instance, _, err := client.GetInstance(name)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf(
			"delete instance %q: outcome unknown after operation wait ended: %v; verification failed: %w",
			name,
			waitErr,
			err,
		)
	}
	if instance.Config["volatile.uuid"] != instanceID {
		return true, nil
	}
	return false, fmt.Errorf(
		"delete instance %q: outcome unknown after operation wait ended: %w; the original instance is still present",
		name,
		waitErr,
	)
}

func deleteInstance(
	ctx context.Context,
	client instanceDeleteClient,
	name string,
	confirm deletionConfirmer,
	output io.Writer,
	cleanup knownHostCleaner,
) error {
	instance, err := fetchDeletionTarget(client, name, "")
	if err != nil {
		return err
	}
	instanceID := instance.Config["volatile.uuid"]
	alias := knownHostAlias(instanceID)

	confirmed, err := confirm(name)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintf(output, "Deletion cancelled; instance %q was not changed.\n", name)
		return nil
	}

	instance, err = fetchDeletionTarget(client, name, instanceID)
	if err != nil {
		return fmt.Errorf("revalidate instance %q after confirmation: %w", name, err)
	}

	if instance.StatusCode == api.Running {
		stopCtx, cancel := context.WithTimeout(ctx, deleteStopWaitTimeout)
		err := updateInstance(
			stopCtx,
			client,
			output,
			name,
			"stop",
			int(deleteStopGracePeriod/time.Second),
		)
		cancel()
		if err != nil {
			return fmt.Errorf("delete instance %q: stop failed: %w", name, err)
		}

		instance, err = fetchDeletionTarget(client, name, instanceID)
		if err != nil {
			return fmt.Errorf("revalidate instance %q after stop: %w", name, err)
		}
		if instance.StatusCode != api.Stopped {
			return fmt.Errorf("instance %q did not reach stopped state; refusing to delete", name)
		}
	}

	fmt.Fprintf(output, "Deleting %s...\n", name)
	op, err := client.DeleteInstance(name)
	if err != nil {
		return fmt.Errorf("delete instance %q: submit delete failed: %w", name, err)
	}

	deleteCtx, cancel := context.WithTimeout(ctx, deleteOperationTimeout)
	err = op.WaitContext(deleteCtx)
	cancel()
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("delete instance %q: delete operation failed: %w", name, err)
		}
		completed, reconcileErr := deletionCompletedAfterWaitError(client, name, instanceID, err)
		if reconcileErr != nil {
			return reconcileErr
		}
		if !completed {
			return fmt.Errorf("delete instance %q: outcome unknown: %w", name, err)
		}
	}

	fmt.Fprintf(output, "Deleted %s.\n", name)
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), knownHostCleanupTimeout)
	defer cleanupCancel()
	if err := cleanup(cleanupCtx, alias); err != nil {
		fmt.Fprintf(output, "Warning: instance %q was deleted, but known-host cleanup failed: %v\n", name, err)
	}
	return nil
}

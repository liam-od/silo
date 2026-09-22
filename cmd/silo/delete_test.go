package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type fakeDeleteOperation struct {
	incus.Operation
	waitErr error
	waited  bool
}

func (o *fakeDeleteOperation) WaitContext(context.Context) error {
	o.waited = true
	return o.waitErr
}

type fakeDeleteClient struct {
	instanceDeleteClient
	instance      *api.Instance
	instances     []*api.Instance
	getErr        error
	getErrs       []error
	getCalls      int
	snapshots     []api.InstanceSnapshot
	snapshotSets  [][]api.InstanceSnapshot
	snapshotCalls int
	backups       []api.InstanceBackup
	backupSets    [][]api.InstanceBackup
	backupCalls   int
	snapshotErr   error
	backupErr     error
	updateErr     error
	deleteErr     error
	updateOp      *fakeDeleteOperation
	deleteOp      *fakeDeleteOperation
	updateAction  string
	updateTimeout int
	deleteName    string
}

func (c *fakeDeleteClient) GetInstance(string) (*api.Instance, string, error) {
	index := c.getCalls
	c.getCalls++
	if index < len(c.getErrs) && c.getErrs[index] != nil {
		return nil, "", c.getErrs[index]
	}
	if c.getErr != nil {
		return nil, "", c.getErr
	}
	if index < len(c.instances) {
		return c.instances[index], "", nil
	}
	return c.instance, "", nil
}

func (c *fakeDeleteClient) GetInstanceSnapshots(string) ([]api.InstanceSnapshot, error) {
	index := c.snapshotCalls
	c.snapshotCalls++
	if index < len(c.snapshotSets) {
		return c.snapshotSets[index], c.snapshotErr
	}
	return c.snapshots, c.snapshotErr
}

func (c *fakeDeleteClient) GetInstanceBackups(string) ([]api.InstanceBackup, error) {
	index := c.backupCalls
	c.backupCalls++
	if index < len(c.backupSets) {
		return c.backupSets[index], c.backupErr
	}
	return c.backups, c.backupErr
}

func (c *fakeDeleteClient) UpdateInstanceState(
	_ string,
	state api.InstanceStatePut,
	_ string,
) (incus.Operation, error) {
	c.updateAction = state.Action
	c.updateTimeout = state.Timeout
	if c.instance != nil && c.updateErr == nil {
		c.instance.StatusCode = api.Stopped
	}
	if c.updateOp == nil {
		c.updateOp = &fakeDeleteOperation{}
	}
	return c.updateOp, c.updateErr
}

func (c *fakeDeleteClient) DeleteInstance(name string) (incus.Operation, error) {
	c.deleteName = name
	if c.deleteOp == nil {
		c.deleteOp = &fakeDeleteOperation{}
	}
	return c.deleteOp, c.deleteErr
}

func testInstance(managed bool, status api.StatusCode) *api.Instance {
	managedValue := "false"
	if managed {
		managedValue = "true"
	}
	return &api.Instance{
		InstancePut: api.InstancePut{Config: map[string]string{
			"user.silo.managed": managedValue,
			"volatile.uuid":     "b6039210-814c-46a1-a062-3dd28c7f136e",
		}},
		Name:       "test-1",
		StatusCode: status,
		Type:       string(api.InstanceTypeVM),
	}
}

func confirmYes(string) (bool, error)         { return true, nil }
func confirmNo(string) (bool, error)          { return false, nil }
func noCleanup(context.Context, string) error { return nil }

func TestAcceptsDeletionConfirmation(t *testing.T) {
	for _, response := range []string{"y", "Y", "yes", "YES", "  yes  "} {
		if !acceptsDeletionConfirmation(response) {
			t.Errorf("response %q was rejected", response)
		}
	}
	for _, response := range []string{"", "n", "no", "true", "delete", "yes please"} {
		if acceptsDeletionConfirmation(response) {
			t.Errorf("response %q was accepted", response)
		}
	}
}

func TestConfirmDeletion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "yes", input: "yes\n", want: true},
		{name: "short yes", input: "Y\n", want: true},
		{name: "default no", input: "\n"},
		{name: "explicit no", input: "no\n"},
		{name: "other response", input: "delete\n"},
		{name: "EOF even after yes", input: "yes"},
		{name: "empty EOF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			got, err := confirmDeletion(strings.NewReader(tt.input), &output, "exact-name")
			if err != nil {
				t.Fatalf("confirmDeletion() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("confirmed = %v, want %v", got, tt.want)
			}
			if !strings.Contains(output.String(), `"exact-name"`) {
				t.Errorf("prompt %q does not contain exact instance name", output.String())
			}
		})
	}
}

func TestInteractiveDeletionConfirmerRejectsNonTerminalInput(t *testing.T) {
	confirm := interactiveDeletionConfirmer(strings.NewReader("yes\n"), io.Discard, false)
	confirmed, err := confirm("test-1")
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") || confirmed {
		t.Fatalf("confirm() = (%v, %v), want non-interactive refusal", confirmed, err)
	}
}

func TestKnownHostCleanupConstruction(t *testing.T) {
	const id = "b6039210-814c-46a1-a062-3dd28c7f136e"
	alias := knownHostAlias(id)
	if alias != "silo-"+id {
		t.Errorf("alias = %q, want %q", alias, "silo-"+id)
	}

	got := buildKnownHostCleanupArgs(alias, "/home/test/.config/silo/known_hosts")
	want := []string{"-R", "silo-" + id, "-f", "/home/test/.config/silo/known_hosts"}
	if !slices.Equal(got, want) {
		t.Errorf("arguments = %v, want %v", got, want)
	}
}

func TestRemoveKnownHostInjectsExactCommand(t *testing.T) {
	knownHosts := t.TempDir() + "/known_hosts"
	if err := os.WriteFile(knownHosts, []byte("entry\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var gotName string
	var gotArgs []string
	err := removeKnownHostFromFile(t.Context(), "silo-id", knownHosts, "/usr/bin/ssh-keygen",
		func(_ context.Context, name string, args ...string) ([]byte, error) {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("removeKnownHostFromFile() error = %v", err)
	}
	if gotName != "/usr/bin/ssh-keygen" {
		t.Errorf("command = %q", gotName)
	}
	wantArgs := []string{"-R", "silo-id", "-f", knownHosts}
	if !slices.Equal(gotArgs, wantArgs) {
		t.Errorf("arguments = %v, want %v", gotArgs, wantArgs)
	}
}

func TestRemoveKnownHostAllowsMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := removeKnownHost(t.Context(), "silo-id"); err != nil {
		t.Fatalf("removeKnownHost() error = %v", err)
	}
}

func TestDeleteInstanceRefusesUnsafeTargets(t *testing.T) {
	tests := []struct {
		name      string
		instance  *api.Instance
		snapshots []api.InstanceSnapshot
		backups   []api.InstanceBackup
		want      string
	}{
		{name: "unmanaged", instance: testInstance(false, api.Stopped), want: "not Silo-managed"},
		{name: "snapshot dependency", instance: testInstance(true, api.Stopped), snapshots: []api.InstanceSnapshot{{}}, want: "snapshot(s)"},
		{name: "backup dependency", instance: testInstance(true, api.Stopped), backups: []api.InstanceBackup{{}}, want: "backup(s)"},
	}

	container := testInstance(true, api.Stopped)
	container.Type = string(api.InstanceTypeContainer)
	tests = append(tests, struct {
		name      string
		instance  *api.Instance
		snapshots []api.InstanceSnapshot
		backups   []api.InstanceBackup
		want      string
	}{name: "container", instance: container, want: "not a VM"})

	missingUUID := testInstance(true, api.Stopped)
	delete(missingUUID.Config, "volatile.uuid")
	tests = append(tests, struct {
		name      string
		instance  *api.Instance
		snapshots []api.InstanceSnapshot
		backups   []api.InstanceBackup
		want      string
	}{name: "missing UUID", instance: missingUUID, want: "has no UUID"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeDeleteClient{instance: tt.instance, snapshots: tt.snapshots, backups: tt.backups}
			err := deleteInstance(t.Context(), client, "test-1", confirmYes, io.Discard, noCleanup)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("deleteInstance() error = %v, want %q", err, tt.want)
			}
			if client.deleteName != "" || client.updateAction != "" {
				t.Fatal("unsafe target caused a destructive action")
			}
		})
	}
}

func TestDeleteInstanceRefusesSnapshotOrBackupName(t *testing.T) {
	client := &fakeDeleteClient{}
	err := deleteInstance(t.Context(), client, "test-1/snap0", confirmYes, io.Discard, noCleanup)
	if err == nil || !strings.Contains(err.Error(), "snapshot or backup") {
		t.Fatalf("deleteInstance() error = %v", err)
	}
	if client.getCalls != 0 || client.deleteName != "" {
		t.Fatal("snapshot-like name reached the Incus API")
	}
}

func TestDeleteInstanceRefusesUnsafeStates(t *testing.T) {
	for _, status := range []api.StatusCode{api.Frozen, api.Freezing, api.Starting, api.Stopping, api.Error} {
		t.Run(status.String(), func(t *testing.T) {
			client := &fakeDeleteClient{instance: testInstance(true, status)}
			err := deleteInstance(t.Context(), client, "test-1", confirmYes, io.Discard, noCleanup)
			if err == nil || !strings.Contains(err.Error(), "only running or stopped") {
				t.Fatalf("deleteInstance() error = %v", err)
			}
			if client.deleteName != "" {
				t.Fatal("unsafe state was deleted")
			}
		})
	}
}

func TestDeleteInstanceCancellationChangesNothing(t *testing.T) {
	client := &fakeDeleteClient{instance: testInstance(true, api.Running)}
	var output bytes.Buffer
	err := deleteInstance(t.Context(), client, "test-1", confirmNo, &output, noCleanup)
	if err != nil {
		t.Fatalf("deleteInstance() error = %v", err)
	}
	if client.deleteName != "" || client.updateAction != "" || client.getCalls != 1 {
		t.Fatal("cancelled deletion caused work after confirmation")
	}
	if !strings.Contains(output.String(), "Deletion cancelled") {
		t.Errorf("output = %q, want cancellation report", output.String())
	}
}

func TestDeleteInstanceRevalidatesTargetAfterConfirmation(t *testing.T) {
	original := testInstance(true, api.Stopped)
	tests := []struct {
		name string
		edit func(*fakeDeleteClient, *api.Instance)
		want string
	}{
		{
			name: "UUID changed",
			edit: func(_ *fakeDeleteClient, instance *api.Instance) {
				instance.Config["volatile.uuid"] = "replacement-id"
			},
			want: "was replaced",
		},
		{
			name: "ownership marker removed",
			edit: func(_ *fakeDeleteClient, instance *api.Instance) {
				instance.Config["user.silo.managed"] = "false"
			},
			want: "not Silo-managed",
		},
		{
			name: "type changed",
			edit: func(_ *fakeDeleteClient, instance *api.Instance) {
				instance.Type = string(api.InstanceTypeContainer)
			},
			want: "not a VM",
		},
		{
			name: "snapshot added",
			edit: func(client *fakeDeleteClient, _ *api.Instance) {
				client.snapshotSets = [][]api.InstanceSnapshot{nil, {{}}}
			},
			want: "snapshot(s)",
		},
		{
			name: "backup added",
			edit: func(client *fakeDeleteClient, _ *api.Instance) {
				client.backupSets = [][]api.InstanceBackup{nil, {{}}}
			},
			want: "backup(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			postConfirmation := testInstance(true, api.Stopped)
			client := &fakeDeleteClient{
				instance:  postConfirmation,
				instances: []*api.Instance{original, postConfirmation},
			}
			tt.edit(client, postConfirmation)

			err := deleteInstance(t.Context(), client, "test-1", confirmYes, io.Discard, noCleanup)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("deleteInstance() error = %v, want %q", err, tt.want)
			}
			if client.deleteName != "" {
				t.Fatal("changed target was deleted")
			}
		})
	}
}

func TestDeleteInstanceStopsRunningVMDeletesAndCleansAlias(t *testing.T) {
	client := &fakeDeleteClient{instance: testInstance(true, api.Running)}
	var output bytes.Buffer
	var cleanedAlias string

	err := deleteInstance(t.Context(), client, "test-1", confirmYes, &output,
		func(_ context.Context, alias string) error {
			cleanedAlias = alias
			return nil
		})
	if err != nil {
		t.Fatalf("deleteInstance() error = %v", err)
	}
	if client.updateAction != "stop" || client.updateTimeout != 90 || client.updateOp == nil || !client.updateOp.waited {
		t.Errorf("stop = action %q, timeout %d, waited %v", client.updateAction, client.updateTimeout, client.updateOp != nil && client.updateOp.waited)
	}
	if client.deleteName != "test-1" || client.deleteOp == nil || !client.deleteOp.waited {
		t.Error("delete operation was not submitted and awaited")
	}
	if cleanedAlias != "silo-b6039210-814c-46a1-a062-3dd28c7f136e" {
		t.Errorf("cleaned alias = %q", cleanedAlias)
	}
	if !strings.Contains(output.String(), "Updating test-1 state: stop") || !strings.Contains(output.String(), "Deleted test-1.") {
		t.Errorf("output = %q, want stop progress and completion", output.String())
	}
}

func TestDeleteInstanceReportsStopFailureWithoutDeleting(t *testing.T) {
	stopFailure := errors.New("stop operation failed")
	client := &fakeDeleteClient{
		instance: testInstance(true, api.Running),
		updateOp: &fakeDeleteOperation{waitErr: stopFailure},
	}

	err := deleteInstance(t.Context(), client, "test-1", confirmYes, io.Discard, noCleanup)
	if !errors.Is(err, stopFailure) || !strings.Contains(err.Error(), "stop failed") {
		t.Fatalf("deleteInstance() error = %v, want stop-stage error", err)
	}
	if client.deleteName != "" {
		t.Error("delete was submitted after the stop failed")
	}
}

func TestDeleteInstanceDoesNotCleanAfterDefinitiveDeleteFailure(t *testing.T) {
	deleteFailure := errors.New("operation failed")
	client := &fakeDeleteClient{
		instance: testInstance(true, api.Stopped),
		deleteOp: &fakeDeleteOperation{waitErr: deleteFailure},
	}
	cleaned := false

	err := deleteInstance(t.Context(), client, "test-1", confirmYes, io.Discard,
		func(context.Context, string) error { cleaned = true; return nil })
	if !errors.Is(err, deleteFailure) || !strings.Contains(err.Error(), "delete operation failed") {
		t.Fatalf("deleteInstance() error = %v, want delete-stage error", err)
	}
	if cleaned {
		t.Error("known-host cleanup ran after failed deletion")
	}
}

func TestDeleteInstanceReconcilesDeleteWaitTimeout(t *testing.T) {
	t.Run("not found means completed", func(t *testing.T) {
		client := &fakeDeleteClient{
			instance: testInstance(true, api.Stopped),
			getErrs:  []error{nil, nil, api.StatusErrorf(http.StatusNotFound, "not found")},
			deleteOp: &fakeDeleteOperation{waitErr: context.DeadlineExceeded},
		}
		cleaned := false
		err := deleteInstance(t.Context(), client, "test-1", confirmYes, io.Discard,
			func(context.Context, string) error { cleaned = true; return nil })
		if err != nil || !cleaned {
			t.Fatalf("deleteInstance() = %v, cleaned = %v", err, cleaned)
		}
	})

	t.Run("still present is unknown", func(t *testing.T) {
		client := &fakeDeleteClient{
			instance: testInstance(true, api.Stopped),
			deleteOp: &fakeDeleteOperation{waitErr: context.DeadlineExceeded},
		}
		err := deleteInstance(t.Context(), client, "test-1", confirmYes, io.Discard, noCleanup)
		if err == nil || !strings.Contains(err.Error(), "outcome unknown") {
			t.Fatalf("deleteInstance() error = %v, want unknown outcome", err)
		}
	})
}

func TestDeleteInstanceCleanupUsesDetachedBoundedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	client := &fakeDeleteClient{instance: testInstance(true, api.Stopped)}
	checked := false

	err := deleteInstance(ctx, client, "test-1", func(string) (bool, error) {
		cancel()
		return true, nil
	}, io.Discard, func(cleanupCtx context.Context, _ string) error {
		checked = true
		if cleanupCtx.Err() != nil {
			t.Errorf("cleanup context inherited cancellation: %v", cleanupCtx.Err())
		}
		deadline, ok := cleanupCtx.Deadline()
		if !ok || time.Until(deadline) > knownHostCleanupTimeout || time.Until(deadline) <= 0 {
			t.Errorf("cleanup deadline = %v, ok = %v", deadline, ok)
		}
		return nil
	})
	if err != nil || !checked {
		t.Fatalf("deleteInstance() = %v, cleanup checked = %v", err, checked)
	}
}

func TestDeleteInstanceWarnsAfterCleanupFailure(t *testing.T) {
	client := &fakeDeleteClient{instance: testInstance(true, api.Stopped)}
	var output bytes.Buffer

	err := deleteInstance(t.Context(), client, "test-1", confirmYes, &output,
		func(context.Context, string) error { return errors.New("ssh-keygen failed") })
	if err != nil {
		t.Fatalf("deleteInstance() error = %v", err)
	}
	if !strings.Contains(output.String(), "was deleted, but known-host cleanup failed") {
		t.Errorf("output = %q, want post-delete cleanup warning", output.String())
	}
}

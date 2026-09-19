package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("missing file returns defaults", func(t *testing.T) {
		got, err := loadConfig(filepath.Join(t.TempDir(), "config.toml"))
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}
		if got != defaultConfig() {
			t.Errorf("config = %#v, want %#v", got, defaultConfig())
		}
	})

	t.Run("partial file retains defaults", func(t *testing.T) {
		path := writeTestConfig(t, `[instance]
ssh_public_key = "/tmp/silo.pub"
`)

		got, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}
		if got.Instance.SSHUser != defaultConfig().Instance.SSHUser {
			t.Errorf("SSH user = %q, want default %q", got.Instance.SSHUser, defaultConfig().Instance.SSHUser)
		}
		if got.Instance.SSHPublicKey != "/tmp/silo.pub" {
			t.Errorf("SSH public key = %q, want %q", got.Instance.SSHPublicKey, "/tmp/silo.pub")
		}
	})

	t.Run("complete file overrides defaults", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		path := writeTestConfig(t, `[instance]
ssh_user = "developer"
ssh_public_key = "~/.ssh/developer.pub"
`)

		got, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}
		if got.Instance.SSHUser != "developer" {
			t.Errorf("SSH user = %q, want %q", got.Instance.SSHUser, "developer")
		}
		wantKey := filepath.Join(home, ".ssh", "developer.pub")
		if got.Instance.SSHPublicKey != wantKey {
			t.Errorf("SSH public key = %q, want %q", got.Instance.SSHPublicKey, wantKey)
		}
	})

	t.Run("malformed file returns error", func(t *testing.T) {
		path := writeTestConfig(t, "[instance\n")
		if _, err := loadConfig(path); err == nil {
			t.Fatal("loadConfig() error = nil, want an error")
		}
	})
}

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "home directory", input: "~", want: home},
		{name: "home-relative path", input: "~/.ssh/silo.pub", want: filepath.Join(home, ".ssh", "silo.pub")},
		{name: "absolute path", input: "/tmp/silo.pub", want: "/tmp/silo.pub"},
		{name: "empty path", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandHome(tt.input)
			if err != nil {
				t.Fatalf("expandHome() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func writeTestConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

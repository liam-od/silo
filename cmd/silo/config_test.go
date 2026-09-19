package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfig(t *testing.T) {
	t.Run("configured key is read", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configDir)

		keyPath := filepath.Join(t.TempDir(), "silo.pub")
		if err := os.WriteFile(keyPath, []byte("ssh-ed25519 key-material test@example\n"), 0o600); err != nil {
			t.Fatalf("write key: %v", err)
		}

		siloDir := filepath.Join(configDir, "silo")
		if err := os.MkdirAll(siloDir, 0o700); err != nil {
			t.Fatalf("create config directory: %v", err)
		}
		contents := "[instance]\nssh_user = \"developer\"\nssh_public_key = \"" + keyPath + "\"\n"
		if err := os.WriteFile(filepath.Join(siloDir, "config.toml"), []byte(contents), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		got, err := getConfig()
		if err != nil {
			t.Fatalf("getConfig() error = %v", err)
		}
		if got.Instance.SSHUser != "developer" {
			t.Errorf("SSH user = %q, want %q", got.Instance.SSHUser, "developer")
		}
		const wantKey = "ssh-ed25519 key-material test@example"
		if got.Instance.SSHPublicKey != wantKey {
			t.Errorf("SSH public key = %q, want %q", got.Instance.SSHPublicKey, wantKey)
		}
	})

	t.Run("missing key configuration returns error", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		if _, err := getConfig(); err == nil {
			t.Fatal("getConfig() error = nil, want an error")
		}
	})
}

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

func TestReadSSHPublicKey(t *testing.T) {
	t.Run("valid content is trimmed", func(t *testing.T) {
		path := writeTestConfig(t, "  ssh-ed25519 key-material test@example  \n")

		got, err := readSSHPublicKey(path)
		if err != nil {
			t.Fatalf("readSSHPublicKey() error = %v", err)
		}
		const want = "ssh-ed25519 key-material test@example"
		if got != want {
			t.Errorf("key = %q, want %q", got, want)
		}
	})

	t.Run("empty file returns error", func(t *testing.T) {
		path := writeTestConfig(t, " \n\t")
		if _, err := readSSHPublicKey(path); err == nil {
			t.Fatal("readSSHPublicKey() error = nil, want an error")
		}
	})

	t.Run("missing file preserves not-exist error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.pub")
		_, err := readSSHPublicKey(path)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("readSSHPublicKey() error = %v, want os.ErrNotExist", err)
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

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type siloConfig struct {
	Instance instanceConfig `toml:"instance"`
}

type instanceConfig struct {
	SSHUser      string `toml:"ssh_user"`
	SSHPublicKey string `toml:"ssh_public_key"`
}

func defaultConfig() siloConfig {
	return siloConfig{
		Instance: instanceConfig{
			SSHUser:      "agent",
			SSHPublicKey: "",
		},
	}
}

func getConfig() (siloConfig, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return defaultConfig(), fmt.Errorf("get user config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "silo", "config.toml")
	config, err := loadConfig(configPath)
	if err != nil {
		return defaultConfig(), err
	}
	if config.Instance.SSHPublicKey == "" {
		return defaultConfig(), fmt.Errorf(
			"instance.ssh_public_key is not configured in %q",
			configPath,
		)
	}

	keyPath := config.Instance.SSHPublicKey
	config.Instance.SSHPublicKey, err = readSSHPublicKey(keyPath)
	if err != nil {
		return defaultConfig(), fmt.Errorf("read SSH public key %q: %w", keyPath, err)
	}

	return config, nil
}

func loadConfig(configPath string) (siloConfig, error) {
	config := defaultConfig()
	_, err := toml.DecodeFile(configPath, &config)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), fmt.Errorf("decode config file %q: %w", configPath, err)
	}

	config.Instance.SSHPublicKey, err = expandHome(config.Instance.SSHPublicKey)
	if err != nil {
		return defaultConfig(), fmt.Errorf("expand SSH public key path: %w", err)
	}

	return config, nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}

	return filepath.Join(home, path[2:]), nil
}

func readSSHPublicKey(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read ssh public key file: %w", err)
	}

	key := strings.TrimSpace(string(content))
	if key == "" {
		return "", fmt.Errorf("empty ssh public key")
	}

	return key, nil
}

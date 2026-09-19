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

	return loadConfig(filepath.Join(configDir, "silo", "config.toml"))
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

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configPathEnv = "SNAGARR_CLI_CONFIG"

type savedConfig struct {
	Server string `json:"server"`
	Token  string `json:"token,omitempty"`
}

type clientOptions struct {
	server  string
	token   string
	json    bool
	noColor bool
}

func configPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv(configPathEnv)); path != "" {
		return path, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config: %w", err)
	}
	return filepath.Join(dir, "snagarr", "client.json"), nil
}

func loadConfig() (savedConfig, string, error) {
	path, err := configPath()
	if err != nil {
		return savedConfig{}, "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return savedConfig{}, path, nil
	}
	if err != nil {
		return savedConfig{}, path, fmt.Errorf("read %s: %w", path, err)
	}
	var config savedConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return savedConfig{}, path, fmt.Errorf("read %s: %w", path, err)
	}
	return config, path, nil
}

func saveConfig(config savedConfig) (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return path, fmt.Errorf("create config directory: %w", err)
	}
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return path, fmt.Errorf("encode config: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return path, fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return path, fmt.Errorf("secure %s: %w", path, err)
	}
	return path, nil
}

func resolveConfig(options clientOptions) (savedConfig, string, error) {
	config, path, err := loadConfig()
	if err != nil {
		return savedConfig{}, path, err
	}
	if value := strings.TrimSpace(os.Getenv("SNAGARR_URL")); value != "" {
		config.Server = value
	}
	if value := strings.TrimSpace(os.Getenv("SNAGARR_TOKEN")); value != "" {
		config.Token = value
	}
	if value := strings.TrimSpace(options.server); value != "" {
		config.Server = value
	}
	if value := strings.TrimSpace(options.token); value != "" {
		config.Token = value
	}
	if strings.TrimSpace(config.Server) == "" {
		return savedConfig{}, path, fmt.Errorf("no server configured; run `snagarr login <server>`")
	}
	return config, path, nil
}

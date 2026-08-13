// Package config holds the bootstrap options Snagarr needs before it can read
// its own database, and the integration settings it keeps inside it.
package config

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Config is what the process needs to start. Everything else is a setting, and
// settings live in the database so the UI can change them.
type Config struct {
	Addr     string
	DataDir  string
	LogLevel slog.Level
}

func Load() Config {
	c := Config{
		Addr:     env("SNAGARR_ADDR", ":8080"),
		DataDir:  env("SNAGARR_DATA_DIR", "data"),
		LogLevel: slog.LevelInfo,
	}
	switch strings.ToLower(env("SNAGARR_LOG_LEVEL", "info")) {
	case "debug":
		c.LogLevel = slog.LevelDebug
	case "warn":
		c.LogLevel = slog.LevelWarn
	case "error":
		c.LogLevel = slog.LevelError
	}
	return c
}

func (c Config) DatabasePath() string { return filepath.Join(c.DataDir, "snagarr.db") }

// SecretKey loads the key that encrypts settings, generating it on first run.
// It sits beside the database, so it protects backups and copies rather than a
// compromised host.
func (c Config) SecretKey() ([]byte, error) {
	path := filepath.Join(c.DataDir, "secret.key")
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("%s must hold 32 bytes, found %d", path, len(key))
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read secret key: %w", err)
	}

	if err := os.MkdirAll(c.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("write secret key: %w", err)
	}
	return key, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

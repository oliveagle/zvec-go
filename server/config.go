// Package server implements a lightweight, dependency-free HTTP service for
// managing one or more zvec collections/tables.
//
// It uses only the Go standard library (net/http, encoding/json, crypto/*).
// Authentication is HTTP Basic, driven by a user list in a JSON config file.
//
// See cmd/zvec-httpd for the runnable entrypoint and config.example.json for
// the configuration schema.
package server

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr                 string `json:"addr"`
	ReadTimeoutSeconds   int    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds  int    `json:"write_timeout_seconds"`
	ShutdownTimeoutSeconds int  `json:"shutdown_timeout_seconds"`
}

// StorageConfig holds where collections are persisted on disk.
type StorageConfig struct {
	DataDir string `json:"data_dir"`
}

// User is a single authenticated principal.
//
// Password may be:
//   - a plain string (used as-is, constant-time compared; a startup warning is
//     emitted recommending a hash), or
//   - "sha256:<64 hex chars>" to store a SHA-256 hash of the password.
//
// Readonly users can read and query but cannot create/modify/drop collections
// or documents.
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Readonly bool   `json:"readonly,omitempty"`
}

// AuthConfig holds the authentication settings.
type AuthConfig struct {
	Enabled bool   `json:"enabled"`
	Users   []User `json:"users"`
}

// Config is the full service configuration, loaded from a JSON file.
type Config struct {
	Server  ServerConfig  `json:"server"`
	Storage StorageConfig `json:"storage"`
	Auth    AuthConfig    `json:"auth"`
}

// DefaultConfig returns a safe default configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:                   "127.0.0.1:8080",
			ReadTimeoutSeconds:     30,
			WriteTimeoutSeconds:    30,
			ShutdownTimeoutSeconds: 10,
		},
		Storage: StorageConfig{DataDir: "./data"},
		Auth:    AuthConfig{Enabled: true},
	}
}

// fillDefaults sets zero-valued fields to their defaults.
func (c *Config) fillDefaults() {
	def := DefaultConfig()
	if c.Server.Addr == "" {
		c.Server.Addr = def.Server.Addr
	}
	if c.Server.ReadTimeoutSeconds <= 0 {
		c.Server.ReadTimeoutSeconds = def.Server.ReadTimeoutSeconds
	}
	if c.Server.WriteTimeoutSeconds <= 0 {
		c.Server.WriteTimeoutSeconds = def.Server.WriteTimeoutSeconds
	}
	if c.Server.ShutdownTimeoutSeconds <= 0 {
		c.Server.ShutdownTimeoutSeconds = def.Server.ShutdownTimeoutSeconds
	}
	if c.Storage.DataDir == "" {
		c.Storage.DataDir = def.Storage.DataDir
	}
}

// LoadConfig reads and parses the JSON configuration at path, applying
// defaults for any unset fields.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	cfg.fillDefaults()
	return cfg, nil
}

// ReadTimeout returns the configured read timeout as a time.Duration.
func (c *Config) ReadTimeout() time.Duration {
	return time.Duration(c.Server.ReadTimeoutSeconds) * time.Second
}

// WriteTimeout returns the configured write timeout as a time.Duration.
func (c *Config) WriteTimeout() time.Duration {
	return time.Duration(c.Server.WriteTimeoutSeconds) * time.Second
}

// ShutdownTimeout returns the configured graceful-shutdown timeout.
func (c *Config) ShutdownTimeout() time.Duration {
	return time.Duration(c.Server.ShutdownTimeoutSeconds) * time.Second
}

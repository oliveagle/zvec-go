// Package zvec provides a Go client for the zvec vector database.
// This package mirrors the Python API design for consistency.
package zvec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Zvec is the main client for zvec operations.
type Zvec struct {
	mu       sync.RWMutex
	config   *Config
	initialized bool
}

var (
	globalZvec *Zvec
	once       sync.Once
)

// Config holds zvec initialization configuration.
type Config struct {
	LogType                 LogType   `json:"log_type,omitempty"`
	LogLevel                LogLevel  `json:"log_level,omitempty"`
	LogDir                  string    `json:"log_dir,omitempty"`
	LogBasename             string    `json:"log_basename,omitempty"`
	LogFileSize             int       `json:"log_file_size,omitempty"` // MB
	LogOverdueDays          int       `json:"log_overdue_days,omitempty"`
	QueryThreads            int       `json:"query_threads,omitempty"`
	OptimizeThreads         int       `json:"optimize_threads,omitempty"`
	InvertToForwardScanRatio float64  `json:"invert_to_forward_scan_ratio,omitempty"`
	BruteForceByKeysRatio   float64   `json:"brute_force_by_keys_ratio,omitempty"`
	MemoryLimitMB           int       `json:"memory_limit_mb,omitempty"`
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		LogType:                 LogTypeConsole,
		LogLevel:                LogLevelWarn,
		LogDir:                  "./logs",
		LogBasename:             "zvec.log",
		LogFileSize:             2048,
		LogOverdueDays:          7,
	}
}

// Init initializes the zvec library with the given configuration.
// This must be called before any other zvec operations.
func Init(cfg *Config) error {
	var err error
	once.Do(func() {
		if cfg == nil {
			cfg = DefaultConfig()
		}

		globalZvec = &Zvec{
			config: cfg,
			initialized: true,
		}

		// Create log directory if needed
		if cfg.LogType == LogTypeFile && cfg.LogDir != "" {
			if err = os.MkdirAll(cfg.LogDir, 0755); err != nil {
				err = fmt.Errorf("failed to create log directory: %w", err)
				return
			}
		}

		// TODO: Initialize actual C++ core via cgo or FFI
		// For now, this is a placeholder that validates the config
	})
	return err
}

// GetInstance returns the global zvec instance.
// Must be called after Init.
func GetInstance() (*Zvec, error) {
	if globalZvec == nil || !globalZvec.initialized {
		return nil, fmt.Errorf("zvec not initialized. Call zvec.Init() first")
	}
	return globalZvec, nil
}

// Config returns the current configuration.
func (z *Zvec) Config() *Config {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.config
}

// ToJSON converts the config to JSON.
func (c *Config) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// CreateAndOpen creates a new collection and opens it.
func CreateAndOpen(path string, schema *CollectionSchema, option *CollectionOption) (*Collection, error) {
	z, err := GetInstance()
	if err != nil {
		return nil, err
	}
	return z.CreateAndOpen(path, schema, option)
}

// Open opens an existing collection.
func Open(path string, option *CollectionOption) (*Collection, error) {
	z, err := GetInstance()
	if err != nil {
		return nil, err
	}
	return z.Open(path, option)
}

// CreateAndOpen creates a new collection and opens it.
func (z *Zvec) CreateAndOpen(path string, schema *CollectionSchema, option *CollectionOption) (*Collection, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	if !z.initialized {
		return nil, fmt.Errorf("zvec not initialized")
	}

	if schema == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}

	if option == nil {
		option = DefaultCollectionOption()
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create collection directory: %w", err)
	}

	// TODO: Actual C++ core integration
	// For now, create a metadata file to simulate creation
	metaPath := filepath.Join(path, "collection.json")
	metaData := map[string]interface{}{
		"name":      schema.Name,
		"schema":    schema,
		"option":    option,
		"created_at": "now",
	}
	metaBytes, err := json.MarshalIndent(metaData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	return &Collection{
		path:   path,
		schema: schema,
		option: option,
		docs:   make(map[string]*Document),
	}, nil
}

// Open opens an existing collection.
func (z *Zvec) Open(path string, option *CollectionOption) (*Collection, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	if !z.initialized {
		return nil, fmt.Errorf("zvec not initialized")
	}

	if option == nil {
		option = DefaultCollectionOption()
	}

	// Check if collection exists
	metaPath := filepath.Join(path, "collection.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("collection not found at %s", path)
	}

	// Read metadata
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metaData struct {
		Name   string           `json:"name"`
		Schema *CollectionSchema `json:"schema"`
		Option *CollectionOption `json:"option"`
	}
	if err := json.Unmarshal(metaBytes, &metaData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &Collection{
		path:   path,
		schema: metaData.Schema,
		option: option,
		docs:   make(map[string]*Document),
	}, nil
}

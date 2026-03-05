package zvec

import (
	"sync"
	"testing"
)

func TestInit(t *testing.T) {
	// Reset global state for testing
	globalZvec = nil
	once = sync.Once{}

	// Test with default config
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig should not return nil")
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	z, err := GetInstance()
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}

	if z.Config().LogType != LogTypeConsole {
		t.Errorf("Expected LogTypeConsole, got %v", z.Config().LogType)
	}
}

func TestConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.LogDir != "./logs" {
		t.Errorf("Expected LogDir ./logs, got %s", cfg.LogDir)
	}

	if cfg.LogFileSize != 2048 {
		t.Errorf("Expected LogFileSize 2048, got %d", cfg.LogFileSize)
	}

	jsonStr, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if jsonStr == "" {
		t.Error("ToJSON returned empty string")
	}
}

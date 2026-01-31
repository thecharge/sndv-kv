package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load defaults: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Port)
	}
	if cfg.KeyCacheSize != 40000 {
		t.Errorf("Expected default cache size 40000, got %d", cfg.KeyCacheSize)
	}
}

func TestLoadFile(t *testing.T) {
	content := `{"port": 9090, "key_cache_size": 100}`
	tmpfile := "test_config.json"
	os.WriteFile(tmpfile, []byte(content), 0644)
	defer os.Remove(tmpfile)

	cfg, err := Load(tmpfile)
	if err != nil {
		t.Fatalf("Failed to load file: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Port)
	}
	if cfg.KeyCacheSize != 100 {
		t.Errorf("Expected cache size 100, got %d", cfg.KeyCacheSize)
	}
}

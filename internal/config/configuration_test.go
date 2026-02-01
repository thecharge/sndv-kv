package config

import (
	"os"
	"testing"
)

func TestLoadConfigurationDefaults(t *testing.T) {
	// Test loading with empty path (defaults)
	config, err := LoadConfigurationFromFile("")
	if err != nil {
		t.Fatalf("Failed to load default configuration: %v", err)
	}

	if config.ServerPort != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.ServerPort)
	}
	if config.KeyCacheCapacityCount != 40000 {
		t.Errorf("Expected default cache capacity 40000, got %d", config.KeyCacheCapacityCount)
	}
}

func TestLoadConfigurationFromFile(t *testing.T) {
	// Create temporary config file
	content := `{
		"server_port": 9090,
		"log_severity_level": "DEBUG"
	}`
	tmpfile := "test_config.json"
	if err := os.WriteFile(tmpfile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile)

	config, err := LoadConfigurationFromFile(tmpfile)
	if err != nil {
		t.Fatalf("Failed to load from file: %v", err)
	}

	if config.ServerPort != 9090 {
		t.Errorf("Expected port 9090, got %d", config.ServerPort)
	}
	if config.LogSeverityLevel != "DEBUG" {
		t.Errorf("Expected log level DEBUG, got %s", config.LogSeverityLevel)
	}
}

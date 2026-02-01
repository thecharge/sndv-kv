package main

import (
	"os"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"testing"
)

func TestRecoverWal(t *testing.T) {
	dir := "./test_main_wal"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	cfg := config.SystemConfiguration{
		WriteAheadLogFilePath: dir + "/wal.log",
		EnableDiskDurability:  true,
	}
	system := core.NewSystemState(cfg)

	// Case 1: Success (New file)
	if err := recoverWal(system); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}
	if system.ActiveWal == nil {
		t.Error("WAL not initialized")
	}
	system.ActiveWal.Close()

	// Case 2: Disabled
	cfg.EnableDiskDurability = false
	system.ActiveWal = nil
	if err := recoverWal(system); err != nil {
		t.Error("Should succeed when disabled")
	}
	if system.ActiveWal != nil {
		t.Error("Should not init WAL when disabled")
	}
}

func TestConfigureRuntime(t *testing.T) {
	cfg := config.SystemConfiguration{MaximumCpuCount: 2}
	configureRuntime(cfg)
}

func TestPrintToken(t *testing.T) {
	cfg := config.SystemConfiguration{AuthenticationSecret: "s"}
	printAdminToken(cfg)

	cfg.AuthenticationToken = "preset"
	printAdminToken(cfg)
}

func TestStartAgents(t *testing.T) {
	dir := "./test_main_agents"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	cfg := config.SystemConfiguration{
		DataDirectoryPath:     dir,
		WriteAheadLogFilePath: dir + "/wal.log",
	}
	sys := core.NewSystemState(cfg)

	startAgents(sys)
}

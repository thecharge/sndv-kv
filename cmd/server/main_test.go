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

	if err := recoverWal(system); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}
	if system.ActiveWal == nil {
		t.Error("WAL not initialized")
	}
	system.ActiveWal.Close()
}

func TestConfigureRuntime(t *testing.T) {
	cfg := config.SystemConfiguration{MaximumCpuCount: 2}
	configureRuntime(cfg)
	// Just ensures no panic
}

func TestPrintToken(t *testing.T) {
	cfg := config.SystemConfiguration{
		AuthenticationSecret: "secret",
	}
	printAdminToken(cfg) // Visual check
}

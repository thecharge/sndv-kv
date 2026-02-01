package agents

import (
	"os"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	"testing"
	"time"
)

func TestIngestionAgentFlow(t *testing.T) {
	// Setup
	dir := "./test_agents"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	logger.InitializeLogger(dir, "ERROR")

	cfg := config.SystemConfiguration{
		DataDirectoryPath:            dir,
		WriteAheadLogFilePath:        dir + "/wal.log",
		MaximumMemtableSizeInBytes:   1024 * 1024,
		EnableDiskDurability:         true,
		BloomFilterFalsePositiveRate: 0.01,
	}

	state := core.NewSystemState(cfg)
	wal, _ := storage.NewDiskWAL(cfg.WriteAheadLogFilePath, true)
	state.ActiveWal = wal

	// Initialize Agents
	InitializeIngestionSubsystem(state)
	StartFlushAgentInBackground(state)

	// Test Ingest
	err := SubmitIngestionRequest("test_key", []byte("test_val"), 0, false)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	// Verify MemTable
	state.Mutex.RLock()
	entry, ok := state.MemTable.Get("test_key")
	state.Mutex.RUnlock()

	if !ok {
		t.Error("Key not found in MemTable after ingest")
	}
	if string(entry.Value) != "test_val" {
		t.Error("Value mismatch")
	}
}

func TestFlushTrigger(t *testing.T) {
	dir := "./test_agent_flush"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	logger.InitializeLogger(dir, "DEBUG")

	cfg := config.SystemConfiguration{
		DataDirectoryPath:            dir,
		WriteAheadLogFilePath:        dir + "/wal.log",
		MaximumMemtableSizeInBytes:   100, // Small limit to force flush
		EnableDiskDurability:         false,
		BloomFilterFalsePositiveRate: 0.01,
	}

	state := core.NewSystemState(cfg)
	InitializeIngestionSubsystem(state)
	StartFlushAgentInBackground(state)

	// Write enough data to trigger rotation
	largeVal := make([]byte, 200)
	SubmitIngestionRequest("k1", largeVal, 0, false)

	// Wait for async flush
	time.Sleep(200 * time.Millisecond)

	state.Mutex.RLock()
	sstCount := len(state.SSTables[0])
	state.Mutex.RUnlock()

	if sstCount == 0 {
		t.Error("Flush agent failed to create SSTable")
	}
}

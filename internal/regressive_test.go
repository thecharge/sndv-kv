package internal

import (
	"fmt"
	"os"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/common"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	"sync"
	"testing"
)

func setupEnv(t *testing.T, opts ...func(*config.SystemConfiguration)) (*core.SystemState, string) {
	dir := "./regress_" + t.Name()
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	logger.InitializeLogger(dir, "DEBUG")

	cfg := config.SystemConfiguration{
		DataDirectoryPath:               dir,
		WriteAheadLogFilePath:           dir + "/wal.log",
		MaximumMemtableSizeInBytes:      1024 * 1024,
		EnableDiskDurability:            true,
		KeyCacheCapacityCount:           1000,
		LevelZeroCompactionTriggerCount: 2,
		CompactionIntervalInSeconds:     1,
		BloomFilterFalsePositiveRate:    0.01,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	bb := core.NewSystemState(cfg)
	w, err := storage.NewDiskWAL(cfg.WriteAheadLogFilePath, true)
	if err != nil {
		t.Fatal(err)
	}
	bb.ActiveWal = w

	agents.InitializeIngestionSubsystem(bb)
	agents.StartFlushAgentInBackground(bb)
	agents.StartCompactionAgentInBackground(bb)
	return bb, dir
}

func TestCrashRecovery(t *testing.T) {
	bb, dir := setupEnv(t)
	defer os.RemoveAll(dir)
	for i := 0; i < 100; i++ {
		agents.SubmitIngestionRequest(fmt.Sprintf("k%d", i), []byte("v"), 0, false)
	}
	bb.ActiveWal.Close()
	w2, _ := storage.NewDiskWAL(dir+"/wal.log", true)
	c := 0
	w2.Replay(func(e common.Entry) { c++ })
	if c != 100 {
		t.Errorf("Recovered %d", c)
	}
}

func TestConcurrentWrites(t *testing.T) {
	bb, dir := setupEnv(t)
	defer os.RemoveAll(dir)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				agents.SubmitIngestionRequest(fmt.Sprintf("w%d_%d", id, j), []byte("x"), 0, false)
			}
		}(i)
	}
	wg.Wait()
	bb.ActiveWal.Close()
	w2, _ := storage.NewDiskWAL(dir+"/wal.log", true)
	c := 0
	w2.Replay(func(e common.Entry) { c++ })
	if c != 500 {
		t.Errorf("Lost writes: %d", c)
	}
}

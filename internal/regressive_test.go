package internal

import (
	"fmt"
	"os"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/common"
	"sndv-kv/internal/storage"
	testFactory "sndv-kv/internal/testing"
	"sync"
	"testing"
)

func TestCrashRecovery_Standard(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	system := f.CreateSystem()
	agents.InitializeIngestionSubsystem(system)

	for i := 0; i < 50; i++ {
		agents.SubmitIngestionRequest(fmt.Sprintf("k%d", i), []byte("v"), 0, false)
	}

	system.ActiveWal.Close()

	wal, err := storage.NewDiskWAL(f.RootDir+"/wal.log", true)
	if err != nil { t.Fatal(err) }

	count := 0
	wal.Replay(func(e common.Entry) { count++ })

	if count != 50 {
		t.Errorf("Recovered %d, expected 50", count)
	}
}

func TestCrashRecovery_CorruptedWal(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	// 1. Setup and Write Valid Data
	system := f.CreateSystem()
	agents.InitializeIngestionSubsystem(system)
	
	agents.SubmitIngestionRequest("valid_1", []byte("val"), 0, false)
	agents.SubmitIngestionRequest("valid_2", []byte("val"), 0, false)
	
	system.ActiveWal.Close()

	// 2. Corrupt the WAL (Append Garbage)
	path := f.RootDir + "/wal.log"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil { t.Fatal(err) }
	
	if _, err := file.Write([]byte{0xFF, 0xFF, 0xFF}); err != nil {
		t.Fatal(err)
	}
	file.Close()

	// 3. Attempt Recovery
	wal, err := storage.NewDiskWAL(path, true)
	if err != nil { t.Fatal(err) }
	defer wal.Close()

	validCount := 0
	// Replay should return error on corruption, but callback should run for valid prefix
	err = wal.Replay(func(e common.Entry) {
		validCount++
	})

	if err == nil {
		t.Error("Expected error from corrupted WAL replay")
	}
	if validCount != 2 {
		t.Errorf("Should recover valid prefix. Got %d, expected 2", validCount)
	}
}

func TestConcurrentWriteIntegrity(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	system := f.CreateSystem()
	agents.InitializeIngestionSubsystem(system)

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

	system.ActiveWal.Close()
	
	wal, _ := storage.NewDiskWAL(f.RootDir+"/wal.log", true)
	count := 0
	wal.Replay(func(e common.Entry) { count++ })
	
	if count != 500 {
		t.Errorf("Lost writes under concurrency. Got %d, expected 500", count)
	}
}
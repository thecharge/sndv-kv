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
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	wal.Replay(func(e common.Entry) { count++ })

	if count != 50 {
		t.Errorf("Recovered %d, expected 50", count)
	}
}

func TestCrashRecovery_CorruptedWal(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	system := f.CreateSystem()
	agents.InitializeIngestionSubsystem(system)

	agents.SubmitIngestionRequest("v1", []byte("val"), 0, false)
	system.ActiveWal.Close()

	// Append garbage
	path := f.RootDir + "/wal.log"
	file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	file.Write([]byte{0xFF, 0xFF, 0xFF})
	file.Close()

	wal, _ := storage.NewDiskWAL(path, true)
	defer wal.Close()

	recovered := 0
	err := wal.Replay(func(e common.Entry) { recovered++ })

	if err == nil {
		t.Error("Expected error on corrupted WAL")
	}
	if recovered != 1 {
		t.Error("Should recover valid prefix")
	}
}

func TestConcurrentIntegrity(t *testing.T) {
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
	c := 0
	wal.Replay(func(e common.Entry) { c++ })

	if c != 500 {
		t.Errorf("Lost writes: %d", c)
	}
}

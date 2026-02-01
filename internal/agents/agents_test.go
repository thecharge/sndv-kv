package agents

import (
	"errors"
	"os"
	"sndv-kv/internal/common"
	"sndv-kv/internal/config"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	testFactory "sndv-kv/internal/testing"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	logger.InitializeLogger("./test_logs_agents", "ERROR")
	code := m.Run()
	os.RemoveAll("./test_logs_agents")
	os.Exit(code)
}

func TestIngest_Success_SingleBatch(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()
	InitializeIngestionSubsystem(state)

	// Critical Path: Single
	if err := SubmitIngestionRequest("k1", []byte("v1"), 0, false); err != nil {
		t.Fatal(err)
	}

	// Critical Path: Batch
	if err := SubmitBatchIngestion([]string{"b1"}, [][]byte{[]byte("v1")}, []int{0}); err != nil {
		t.Fatal(err)
	}

	state.Mutex.RLock()
	_, ok1 := state.MemTable.Get("k1")
	_, ok2 := state.MemTable.Get("b1")
	state.Mutex.RUnlock()

	if !ok1 || !ok2 {
		t.Error("Data missing")
	}
}

func TestIngest_Negative_WalError(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()
	InitializeIngestionSubsystem(state)

	state.ActiveWal.Close() // Sabotage

	if err := SubmitIngestionRequest("k", []byte("v"), 0, false); err == nil {
		t.Error("Expected WAL error")
	}
}

func TestIngest_Positive_Rotation(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 10
	})
	InitializeIngestionSubsystem(state)

	SubmitIngestionRequest("trigger", make([]byte, 20), 0, false)

	for i := 0; i < 20; i++ {
		state.Mutex.RLock()
		rotated := len(state.ImmutableMem) > 0
		state.Mutex.RUnlock()
		if rotated {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("Rotation failed")
}

func TestIngest_Negative_RotationFailure(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 10
	})

	initialWal := state.ActiveWal
	state.Configuration.WriteAheadLogFilePath = f.RootDir + "/missing/wal" // Sabotage path

	rotateWal(state)

	if state.ActiveWal != initialWal {
		t.Error("ActiveWal changed on failure")
	}
}

func TestFlush_Positive_Cycle(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()
	StartFlushAgentInBackground(state)

	mem := storage.NewMemoryTable(100)
	mem.Put("f", []byte("v"), 0, false)

	state.Mutex.Lock()
	state.ImmutableMem = append(state.ImmutableMem, mem)
	state.FlushCondition.Signal()
	state.Mutex.Unlock()

	for i := 0; i < 20; i++ {
		state.Mutex.RLock()
		flushed := len(state.SSTables) > 0 && len(state.SSTables[0]) > 0
		state.Mutex.RUnlock()
		if flushed {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("Flush failed")
}

func TestFlush_Negative_CommitError(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()

	commitFlush(state, storage.SSTableMetadata{}, errors.New("err"), "f", 0)

	state.Mutex.RLock()
	if len(state.SSTables[0]) != 0 {
		t.Error("Committed on error")
	}
	state.Mutex.RUnlock()
}

func TestCompaction_Positive_Merge(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.LevelZeroCompactionTriggerCount = 2
		c.CompactionIntervalInSeconds = 1
	})
	StartCompactionAgentInBackground(state)

	e := []common.Entry{{Key: "c", Value: []byte("v")}}
	m1, _ := storage.WriteSortedStringTableToDisk(e, f.RootDir+"/1.sst", 0, nil)
	m2, _ := storage.WriteSortedStringTableToDisk(e, f.RootDir+"/2.sst", 0, nil)

	state.Mutex.Lock()
	if len(state.SSTables) == 0 {
		state.SSTables = make([][]storage.SSTableMetadata, 4)
	}
	state.SSTables[0] = append(state.SSTables[0], m1, m2)
	state.Mutex.Unlock()

	for i := 0; i < 30; i++ {
		state.Mutex.RLock()
		merged := len(state.SSTables) > 1 && len(state.SSTables[1]) > 0
		state.Mutex.RUnlock()
		if merged {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Error("Compaction failed")
}

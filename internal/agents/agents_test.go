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

// -----------------------------------------------------------------------------
// Ingestion Agent Tests
// -----------------------------------------------------------------------------

func TestIngest_SubmitSingle_CriticalPath(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()
	InitializeIngestionSubsystem(state)

	if err := SubmitIngestionRequest("k1", []byte("v1"), 0, false); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	state.Mutex.RLock()
	val, ok := state.MemTable.Get("k1")
	state.Mutex.RUnlock()

	if !ok || string(val.Value) != "v1" {
		t.Error("Ingestion failed to update MemTable")
	}
}

func TestIngest_SubmitBatch_CriticalPath(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()
	InitializeIngestionSubsystem(state)

	keys := []string{"b1", "b2"}
	vals := [][]byte{[]byte("v1"), []byte("v2")}
	ttls := []int{0, 0}

	if err := SubmitBatchIngestion(keys, vals, ttls); err != nil {
		t.Fatalf("Batch failed: %v", err)
	}

	state.Mutex.RLock()
	_, ok1 := state.MemTable.Get("b1")
	_, ok2 := state.MemTable.Get("b2")
	state.Mutex.RUnlock()

	if !ok1 || !ok2 {
		t.Error("Batch ingestion failed to update MemTable")
	}
}

func TestIngest_Negative_BatchEmpty(t *testing.T) {
	if err := SubmitBatchIngestion(nil, nil, nil); err != nil {
		t.Error("Empty batch should return nil")
	}
}

func TestIngest_Negative_WalError(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	state := f.CreateSystem()
	InitializeIngestionSubsystem(state)

	// Sabotage WAL
	state.ActiveWal.Close()

	err := SubmitIngestionRequest("k1", []byte("v1"), 0, false)
	if err == nil {
		t.Error("Expected error from closed WAL")
	}
}

func TestIngest_Positive_RotationTrigger(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 10
	})
	InitializeIngestionSubsystem(state)

	SubmitIngestionRequest("trigger", make([]byte, 20), 0, false)

	for i := 0; i < 20; i++ {
		state.Mutex.RLock()
		count := len(state.ImmutableMem)
		state.Mutex.RUnlock()
		if count > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("Memtable did not rotate")
}

func TestIngest_Negative_RotationWalFailure(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	// 1. Valid init
	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 10
	})

	initialWal := state.ActiveWal

	// 2. Sabotage: Set path to a non-existent directory/file logic that fails OpenFile
	state.Configuration.WriteAheadLogFilePath = f.RootDir + "/missing_dir/wal.log"

	// 3. Trigger rotation
	rotateWal(state)

	// 4. Assert: ActiveWal should NOT change on failure
	if state.ActiveWal != initialWal {
		t.Error("ActiveWal should not change if rotation fails")
	}
}

// Coverage for helper functions
func TestHelper_PrepareEntries(t *testing.T) {
	req := IngestReq{Key: "k", TTL: 10}
	batch := []IngestReq{req}

	// FIX: Pass nil for the reusable buffer argument
	entries := prepareEntries(batch, nil)

	if entries[0].ExpiryTimestamp == 0 {
		t.Error("TTL calculation failed")
	}
}

func TestHelper_DrainQueue(t *testing.T) {
	queue := make(chan *IngestReq, 200)
	for i := 0; i < 150; i++ {
		queue <- &IngestReq{}
	}
	var batch []IngestReq
	drainSingleQueue(queue, &batch)
	if len(batch) != 100 {
		t.Errorf("Expected 100, got %d", len(batch))
	}
}

// -----------------------------------------------------------------------------
// Flush Agent Tests
// -----------------------------------------------------------------------------

func TestFlush_CriticalPath_Cycle(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()
	StartFlushAgentInBackground(state)

	mem := storage.NewMemoryTable(100)
	mem.Put("f1", []byte("v"), 0, false)

	state.Mutex.Lock()
	state.ImmutableMem = append(state.ImmutableMem, mem)
	state.FlushCondition.Signal()
	state.Mutex.Unlock()

	for i := 0; i < 20; i++ {
		state.Mutex.RLock()
		l0 := len(state.SSTables) > 0 && len(state.SSTables[0]) > 0
		state.Mutex.RUnlock()
		if l0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("Flush failed to create SSTable")
}

func TestFlush_Negative_CommitError(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()

	// Direct call to test error branch
	commitFlush(state, storage.SSTableMetadata{}, errors.New("err"), "f", 0)

	state.Mutex.RLock()
	if len(state.SSTables[0]) != 0 {
		t.Error("Committed despite error")
	}
	state.Mutex.RUnlock()
}

func TestFlush_Positive_RotateFrozen(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()

	// Mock frozen WAL
	wal, _ := storage.NewDiskWAL(f.RootDir+"/frozen.wal", true)
	state.FrozenWALs = append(state.FrozenWALs, wal)

	rotateFrozenWal(state)

	if len(state.FrozenWALs) != 0 {
		t.Error("Frozen WAL not removed")
	}
	if _, err := os.Stat(f.RootDir + "/frozen.wal"); !os.IsNotExist(err) {
		t.Error("Frozen WAL file not deleted")
	}
}

// -----------------------------------------------------------------------------
// Compaction Agent Tests
// -----------------------------------------------------------------------------

func TestCompaction_CriticalPath_Merge(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.LevelZeroCompactionTriggerCount = 2
		c.CompactionIntervalInSeconds = 1
	})
	StartCompactionAgentInBackground(state)

	e := []common.Entry{{Key: "c", Value: []byte("v")}}
	m1, _ := storage.WriteSortedStringTableToDisk(e, f.RootDir+"/L0_1.sst", 0, nil)
	m2, _ := storage.WriteSortedStringTableToDisk(e, f.RootDir+"/L0_2.sst", 0, nil)

	state.Mutex.Lock()
	if len(state.SSTables) == 0 {
		state.SSTables = make([][]storage.SSTableMetadata, 4)
	}
	state.SSTables[0] = append(state.SSTables[0], m1, m2)
	state.Mutex.Unlock()

	for i := 0; i < 30; i++ {
		state.Mutex.RLock()
		l1 := len(state.SSTables) > 1 && len(state.SSTables[1]) > 0
		state.Mutex.RUnlock()
		if l1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Error("Compaction failed")
}

func TestCompaction_Negative_MergeError(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	// Create invalid metadata pointing to non-existent file
	badMeta := storage.SSTableMetadata{Filename: "missing.sst"}

	_, _, err := performMerge([]storage.SSTableMetadata{badMeta}, f.RootDir, nil)
	if err == nil {
		t.Error("Expected error opening missing SSTable")
	}
}

func TestCompaction_EdgeCase_TombstonePreservation(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()

	e1 := []common.Entry{{Key: "k", Value: []byte("v"), ExpiryTimestamp: 0}}
	e2 := []common.Entry{{Key: "k", Value: nil, IsDeleted: true}}

	m1, _ := storage.WriteSortedStringTableToDisk(e1, f.RootDir+"/1.sst", 0, nil)
	m2, _ := storage.WriteSortedStringTableToDisk(e2, f.RootDir+"/2.sst", 0, nil)

	fname, _, err := performMerge([]storage.SSTableMetadata{m1, m2}, f.RootDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	reader, _ := storage.NewSSTableReader(fname)
	entry, _ := reader.Next()
	reader.Close()

	if !entry.IsDeleted {
		t.Error("Deleted state lost during merge")
	}
}

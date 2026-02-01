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
// Ingestion Tests
// -----------------------------------------------------------------------------

func TestIngest_SubmitSingle_Success(t *testing.T) {
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

func TestIngest_SubmitBatch_Success(t *testing.T) {
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

func TestIngest_Batch_Empty(t *testing.T) {
	if err := SubmitBatchIngestion(nil, nil, nil); err != nil {
		t.Error("Empty batch should return nil")
	}
}

func TestIngest_WalError_TriggersNotifyError(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	
	// Create system with durability
	state := f.CreateSystem()
	InitializeIngestionSubsystem(state)
	
	// Sabotage the WAL to force error
	state.ActiveWal.Close()

	err := SubmitIngestionRequest("k1", []byte("v1"), 0, false)
	if err == nil {
		t.Error("Expected error from closed WAL")
	}
}

func TestIngest_Rotation_Triggers(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	
	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 10 // Very small to force rotate
	})
	InitializeIngestionSubsystem(state)

	SubmitIngestionRequest("trigger", make([]byte, 20), 0, false)

	// Spin wait for rotation
	for i := 0; i < 20; i++ {
		state.Mutex.RLock()
		count := len(state.ImmutableMem)
		state.Mutex.RUnlock()
		if count > 0 { return }
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("Memtable did not rotate")
}

func TestIngest_Rotation_WalFailure(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	
	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 10
		// Point WAL to invalid path to cause rotation failure
		c.WriteAheadLogFilePath = f.RootDir // Directory, not file
	})
	
	// Manually set a valid WAL first so system starts
	validWal, _ := storage.NewDiskWAL(f.RootDir+"/initial.wal", true)
	state.ActiveWal = validWal
	
	rotateWal(state) // Call directly to verify logging/handling
	
	// If it failed, ActiveWal is unchanged (still the initial one)
	if state.ActiveWal != validWal {
		t.Error("ActiveWal should not change on failure")
	}
}

// -----------------------------------------------------------------------------
// Flush Agent Tests
// -----------------------------------------------------------------------------

func TestFlushAgent_SuccessfulFlush(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()
	StartFlushAgentInBackground(state)

	// Inject immutable memtable
	mem := storage.NewMemoryTable(100)
	mem.Put("f1", []byte("v"), 0, false)
	
	state.Mutex.Lock()
	state.ImmutableMem = append(state.ImmutableMem, mem)
	state.FlushCondition.Signal()
	state.Mutex.Unlock()

	// Wait for SST
	for i := 0; i < 20; i++ {
		state.Mutex.RLock()
		l0 := len(state.SSTables) > 0 && len(state.SSTables[0]) > 0
		state.Mutex.RUnlock()
		if l0 { return }
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("Flush failed to create SSTable")
}

func TestFlushAgent_CommitLogic(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem()

	// Mocking commitFlush call directly to test logic
	meta := storage.SSTableMetadata{Filename: "mock.sst"}
	
	// Case: Error
	commitFlush(state, meta, errors.New("fail"), "mock.sst", 0)
	state.Mutex.RLock()
	if len(state.SSTables[0]) != 0 {
		t.Error("Should not commit on error")
	}
	state.Mutex.RUnlock()

	// Case: Success
	commitFlush(state, meta, nil, "mock.sst", 1)
	state.Mutex.RLock()
	if len(state.SSTables[0]) != 1 {
		t.Error("Should commit success")
	}
	state.Mutex.RUnlock()
}

// -----------------------------------------------------------------------------
// Compaction Agent Tests
// -----------------------------------------------------------------------------

func TestCompaction_TriggerAndMerge(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.LevelZeroCompactionTriggerCount = 2
		c.CompactionIntervalInSeconds = 1
	})
	StartCompactionAgentInBackground(state)

	// Create 2 L0 tables
	e := []common.Entry{{Key: "c1", Value: []byte("v")}}
	m1, _ := storage.WriteSortedStringTableToDisk(e, f.RootDir+"/L0_1.sst", 0, nil)
	m2, _ := storage.WriteSortedStringTableToDisk(e, f.RootDir+"/L0_2.sst", 0, nil)

	state.Mutex.Lock()
	if len(state.SSTables) == 0 { state.SSTables = make([][]storage.SSTableMetadata, 4) }
	state.SSTables[0] = append(state.SSTables[0], m1, m2)
	state.Mutex.Unlock()

	// Wait for merge
	for i := 0; i < 30; i++ {
		state.Mutex.RLock()
		l1 := len(state.SSTables) > 1 && len(state.SSTables[1]) > 0
		state.Mutex.RUnlock()
		if l1 { return }
		time.Sleep(100 * time.Millisecond)
	}
	t.Error("Compaction failed")
}

func TestCompaction_MergeLogic_HandlesDeleted(t *testing.T) {
	f := testFactory.NewTestFactory(t)
	defer f.Cleanup()
	
	// Create tables with overwrite/delete
	e1 := []common.Entry{{Key: "k1", Value: []byte("v1"), ExpiryTimestamp: 0}}
	e2 := []common.Entry{{Key: "k1", Value: nil, IsDeleted: true}}
	
	m1, _ := storage.WriteSortedStringTableToDisk(e1, f.RootDir+"/1.sst", 0, nil)
	m2, _ := storage.WriteSortedStringTableToDisk(e2, f.RootDir+"/2.sst", 0, nil)
	
	tables := []storage.SSTableMetadata{m1, m2}
	
	// Perform Merge
	fname, _, err := performMerge(tables, f.RootDir, nil)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}
	
	// Verify result contains the deletion (latest version)
	reader, _ := storage.NewSSTableReader(fname)
	entry, ok := reader.Next()
	reader.Close()
	
	if !ok || !entry.IsDeleted {
		t.Error("Merge did not preserve latest deleted state")
	}
}
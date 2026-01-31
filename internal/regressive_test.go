package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/api"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	"sync"
	"testing"
	"time"
)

func setupEnv(t *testing.T, opts ...func(*config.Config)) (*core.Blackboard, string) {
	dir := "./regress_" + t.Name()
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	
	logger.Init(dir, "DEBUG")

	cfg := config.Config{
		DataDir:             dir,
		WalPath:             dir + "/wal.log",
		MaxMemTableSize:     1024 * 1024,
		Durability:          true,
		KeyCacheSize:        1000,
		L0CompactionTrigger: 2,
		CompactionInterval:  1,
		BloomFPR:            0.01,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	bb := core.NewBlackboard(cfg)
	w, err := storage.OpenWAL(cfg.WalPath, true)
	if err != nil { t.Fatal(err) }
	bb.ActiveWal = w

	agents.InitIngest(bb)
	agents.StartFlushAgent(bb)
	agents.StartCompactionAgent(bb)

	return bb, dir
}

// 1. Crash Recovery
func TestCrashRecovery(t *testing.T) {
	bb, dir := setupEnv(t)
	defer os.RemoveAll(dir)
	target := 100
	for i := 0; i < target; i++ { agents.SubmitIngest(fmt.Sprintf("k%d", i), []byte("v"), 0, false) }
	bb.ActiveWal.Close()
	w2, _ := storage.OpenWAL(dir+"/wal.log", true)
	c := 0
	w2.Replay(func(k string, v []byte, e int64, d bool) { c++ })
	if c != target { t.Errorf("Expected %d, got %d", target, c) }
}

// 2. WAL Rotation
func TestWALRotationWorks(t *testing.T) {
	bb, dir := setupEnv(t, func(c *config.Config) { c.MaxMemTableSize = 50 })
	defer os.RemoveAll(dir)
	agents.SubmitIngest("heavy", make([]byte, 100), 0, false)
	time.Sleep(200 * time.Millisecond)
	bb.Mu.RLock()
	if len(bb.SSTables[0]) == 0 { t.Error("No SSTable") }
	bb.Mu.RUnlock()
}

// 3. Concurrent Writes
func TestConcurrentWrites(t *testing.T) {
	bb, dir := setupEnv(t)
	defer os.RemoveAll(dir)
	var wg sync.WaitGroup
	workers := 10
	ops := 50
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				agents.SubmitIngest(fmt.Sprintf("w%d_%d", id, j), []byte("x"), 0, false)
			}
		}(i)
	}
	wg.Wait()
	
	// Verify count via Replay since internal maps are async flushing
	bb.ActiveWal.Close()
	w2, _ := storage.OpenWAL(dir+"/wal.log", true)
	c := 0
	w2.Replay(func(k string, v []byte, e int64, d bool) { c++ })
	if c != workers*ops { t.Errorf("Lost writes. Expected %d, got %d", workers*ops, c) }
}

// 4. Log Rotation
func TestLogRotation(t *testing.T) {
	dir := "./test_logs_rotate_content"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)
	
	logger.Init(dir, "INFO")
	logger.Info("MARKER_BEFORE_ROTATION")
	time.Sleep(50 * time.Millisecond)
	logger.Shutdown()
	
	err := os.Rename(dir+"/system.log", dir+"/system.log.bak")
	if err != nil { t.Fatalf("Rename failed: %v", err) }
	
	logger.Init(dir, "INFO")
	logger.Info("MARKER_AFTER_ROTATION")
	time.Sleep(50 * time.Millisecond)
	logger.Shutdown()
	
	contentOld, _ := os.ReadFile(dir + "/system.log.bak")
	contentNew, _ := os.ReadFile(dir + "/system.log")
	
	if !bytes.Contains(contentOld, []byte("MARKER_BEFORE_ROTATION")) { t.Error("Backup log missing data") }
	if !bytes.Contains(contentNew, []byte("MARKER_AFTER_ROTATION")) { t.Error("New log missing data") }
}

// 5. Compaction Integrity
func TestCompactionIntegrity(t *testing.T) {
	bb, dir := setupEnv(t)
	defer os.RemoveAll(dir)
	entries := []storage.Entry{{Key: "a", Value: []byte("val")}}
	m1, _ := storage.WriteSSTable(entries, dir+"/L0_1.sst", 0, bb.SharedBloom)
	m2, _ := storage.WriteSSTable(entries, dir+"/L0_2.sst", 0, bb.SharedBloom)
	m3, _ := storage.WriteSSTable(entries, dir+"/L0_3.sst", 0, bb.SharedBloom)
	bb.Mu.Lock()
	bb.SSTables[0] = []storage.SSTableMetadata{m1, m2, m3}
	bb.Mu.Unlock()
	time.Sleep(2500 * time.Millisecond)
	bb.Mu.RLock()
	l1 := len(bb.SSTables[1])
	bb.Mu.RUnlock()
	if l1 == 0 { t.Error("Compaction failed") }
}

// 6. Flush Critical Path
func TestFlushCritical(t *testing.T) {
	bb, dir := setupEnv(t)
	defer os.RemoveAll(dir)
	agents.SubmitIngest("k1", []byte("v"), 0, false)
	bb.Mu.Lock()
	bb.ImmutableMem = append(bb.ImmutableMem, bb.MemTable)
	bb.MemTable = storage.NewSwissTable(1024)
	bb.FlushCond.Signal()
	bb.Mu.Unlock()
	time.Sleep(200 * time.Millisecond)
	bb.Mu.RLock()
	if len(bb.SSTables[0]) == 0 { t.Error("Flush failed") }
	bb.Mu.RUnlock()
}

// 7. API Batch
func TestAPIBatchRetrieval(t *testing.T) {
	bb, dir := setupEnv(t)
	defer os.RemoveAll(dir)
	router := &api.Router{BB: bb}
	type Item struct { Key, Value string; TTL int }
	items := make([]Item, 50)
	for i:=0; i<50; i++ { items[i] = Item{fmt.Sprintf("b_%d", i), "v", 0} }
	b, _ := json.Marshal(map[string]interface{}{"items":items})
	w := httptest.NewRecorder()
	router.HandleBatchPut(w, httptest.NewRequest("POST", "/batch", bytes.NewBuffer(b)))
	if w.Code != 201 { t.Fatal("Batch failed") }
	w2 := httptest.NewRecorder()
	router.HandleGet(w2, httptest.NewRequest("GET", "/get?key=b_10", nil))
	if w2.Code != 200 { t.Error("Get failed") }
}

// 8. WAL Corruption
func TestWALCorruption(t *testing.T) {
	dir := "./test_chaos_wal"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)
	p := dir + "/c.wal"
	w, _ := storage.OpenWAL(p, true)
	w.AppendBatch([]storage.Entry{{Key: "ok", Value: []byte("v")}})
	w.Close()
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0644)
	f.Write([]byte{0xFF})
	f.Close()
	w2, _ := storage.OpenWAL(p, true)
	c := 0
	w2.Replay(func(k string, v []byte, e int64, d bool) { c++ })
	if c != 1 { t.Error("Lost valid data") }
}

// 9. Tombstone Logic
func TestDeleteTombstone(t *testing.T) {
	bb, dir := setupEnv(t)
	defer os.RemoveAll(dir)
	router := &api.Router{BB: bb}
	agents.SubmitIngest("del", []byte("v"), 0, false)
	agents.SubmitIngest("del", nil, 0, true)
	w := httptest.NewRecorder()
	router.HandleGet(w, httptest.NewRequest("GET", "/get?key=del", nil))
	if w.Code != 404 { t.Error("Not 404") }
}

// 10. Large Key Handling
func TestLargeKeyHandling(t *testing.T) {
	bb, dir := setupEnv(t, func(c *config.Config) { c.MaxMemTableSize = 5 * 1024 * 1024 })
	defer os.RemoveAll(dir)
	val := make([]byte, 1024*1024) 
	val[0] = 'A'
	err := agents.SubmitIngest("large", val, 0, false)
	if err != nil { t.Fatal(err) }
	bb.Mu.RLock()
	entry, ok := bb.MemTable.Get("large")
	bb.Mu.RUnlock()
	if !ok { t.Fatal("Large key not in MemTable") }
	if len(entry.Value) != len(val) { t.Error("Size mismatch") }
}
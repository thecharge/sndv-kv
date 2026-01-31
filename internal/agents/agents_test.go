package agents

import (
	"os"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"testing"
	"time"
)

func TestIngestAndFlush(t *testing.T) {
	os.MkdirAll("./test_agent_data", 0755)
	defer os.RemoveAll("./test_agent_data")

	cfg := config.Config{
		DataDir:         "./test_agent_data",
		WalPath:         "./test_agent_data/wal.log",
		MaxMemTableSize: 1024,
		Durability:      false,
		KeyCacheSize:    1000,
	}

	bb := core.NewBlackboard(cfg)

	InitIngest(bb)
	StartFlushAgent(bb)

	key := "agent_test_key"
	val := []byte("value")

	err := SubmitIngest(key, val, 0, false)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	bb.Mu.RLock()
	entry, ok := bb.MemTable.Get(key)
	bb.Mu.RUnlock()

	if !ok {
		t.Fatal("Key not found in MemTable")
	}
	if string(entry.Value) != string(val) {
		t.Errorf("Value mismatch")
	}

	time.Sleep(100 * time.Millisecond)
}

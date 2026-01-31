package storage

import (
	"os"
	"testing"
)

func TestWALBatchAndRecovery(t *testing.T) {
	fname := "test_batch.wal"
	defer os.Remove(fname)

	// 1. Open
	wal, err := OpenWAL(fname, true)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}

	// 2. Write Batch
	entries := []Entry{
		{Key: "k1", Value: []byte("v1"), ExpiresAt: 100, Deleted: false},
		{Key: "k2", Value: []byte("v2"), ExpiresAt: 200, Deleted: true},
	}
	if err := wal.AppendBatch(entries); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	wal.Close()

	// 3. Replay
	wal2, err := OpenWAL(fname, false)
	if err != nil {
		t.Fatalf("Re-Open: %v", err)
	}

	found := make(map[string]Entry)
	wal2.Replay(func(k string, v []byte, exp int64, del bool) {
		found[k] = Entry{Key: k, Value: v, ExpiresAt: exp, Deleted: del}
	})

	// 4. Verify
	if e, ok := found["k1"]; !ok || string(e.Value) != "v1" {
		t.Error("k1 corrupted or missing")
	}
	if e, ok := found["k2"]; !ok || !e.Deleted {
		t.Error("k2 tombstone corrupted")
	}
}

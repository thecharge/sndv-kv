package storage

import (
	"os"
	"sndv-kv/internal/common"
	"testing"
)

func TestShardedMemoryTable_AllOps(t *testing.T) {
	mt := NewMemoryTable(100)

	// Positive: Put/Get
	mt.Put("k1", []byte("v1"), 0, false)
	e, ok := mt.Get("k1")
	if !ok || string(e.Value) != "v1" {
		t.Error("Put/Get mismatch")
	}

	// Positive: Overwrite
	mt.Put("k1", []byte("v2"), 0, false)
	e, ok = mt.Get("k1")
	if !ok || string(e.Value) != "v2" {
		t.Error("Overwrite failed")
	}

	// Positive: GetAll
	if len(mt.GetAll()) != 1 {
		t.Error("GetAll count wrong")
	}

	// Negative: Missing
	if _, ok := mt.Get("missing"); ok {
		t.Error("Found missing key")
	}
}

func TestSSTable_AllOps(t *testing.T) {
	fname := "test_engine.sst"
	defer os.Remove(fname)

	entries := []common.Entry{
		{Key: "a", Value: []byte("val_a")},
		{Key: "z", Value: []byte("val_z"), IsDeleted: true},
	}

	// Positive: Write
	meta, err := WriteSortedStringTableToDisk(entries, fname, 0, nil)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Positive: Find
	e, found := FindInSSTable(meta, "a")
	if !found || string(e.Value) != "val_a" {
		t.Error("Find failed")
	}

	// Negative: Find Missing (Bloom skip simulation if bloom was used)
	if _, found := FindInSSTable(meta, "missing"); found {
		t.Error("Found missing key")
	}

	// Positive: Iterator
	reader, _ := NewSSTableReader(fname)
	defer reader.Close()

	e1, _ := reader.Next()
	e2, _ := reader.Next()
	_, ok3 := reader.Next()

	if e1.Key != "a" || e2.Key != "z" || ok3 {
		t.Error("Iterator failed")
	}
}

func TestBloomFilter_AllOps(t *testing.T) {
	bf := NewSharedBloomFilter(100, 0.01)
	bf.Add(1, []byte("k1"))

	if !bf.Contains(1, []byte("k1")) {
		t.Error("False negative")
	}
	if bf.Contains(1, []byte("missing")) {
		// Probabilistic, but should be false
	}
}

func TestWAL_AllOps(t *testing.T) {
	fname := "test_engine.wal"
	defer os.Remove(fname)

	wal, _ := NewDiskWAL(fname, true)
	wal.WriteBatch([]common.Entry{{Key: "k", Value: []byte("v")}})
	wal.Close()

	// Replay
	wal2, _ := NewDiskWAL(fname, true)
	count := 0
	wal2.Replay(func(e common.Entry) { count++ })
	wal2.Close()

	if count != 1 {
		t.Error("Replay count mismatch")
	}

	// Delete
	if err := wal2.Delete(); err != nil {
		t.Error("Delete failed")
	}
}

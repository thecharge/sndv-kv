package storage

import (
	"os"
	"sndv-kv/internal/common"
	"testing"
)

func TestBloomFilter(t *testing.T) {
	bf := NewSharedBloomFilter(100, 0.01)
	if bf.hashCount == 0 {
		t.Error("Hash count zero")
	}
	bf.Add(1, []byte("key"))
	if !bf.Contains(1, []byte("key")) {
		t.Error("False negative")
	}
}

func TestWAL(t *testing.T) {
	fname := "test.wal"
	defer os.Remove(fname)

	wal, _ := NewDiskWAL(fname, true)
	wal.WriteBatch([]common.Entry{{Key: "k", Value: []byte("v")}})
	wal.Close()

	wal2, _ := NewDiskWAL(fname, true)
	count := 0
	wal2.Replay(func(e common.Entry) {
		if e.Key == "k" {
			count++
		}
	})
	if count != 1 {
		t.Error("Replay failed")
	}
	wal2.Close()
}

func TestSSTable(t *testing.T) {
	fname := "L0_1.sst"
	defer os.Remove(fname)
	entries := []common.Entry{{Key: "k", Value: []byte("v")}}
	meta, _ := WriteSortedStringTableToDisk(entries, fname, 0, nil)
	entry, found := FindInSSTable(meta, "k")
	if !found || string(entry.Value) != "v" {
		t.Error("SSTable read failed")
	}
}

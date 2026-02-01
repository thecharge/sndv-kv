package storage

import (
	"fmt"
	"testing"
)

func TestBloomFilterCalculus(t *testing.T) {
	bf := NewSharedBloomFilter(1000, 0.01)
	if bf.hashCount == 0 {
		t.Error("Hash count zero")
	}
	if len(bf.shards) != 32 {
		t.Error("Expected 32 shards")
	}
}

func TestBloomFilterBasic(t *testing.T) {
	bf := NewSharedBloomFilter(100, 0.01)
	key := []byte("key1")
	id := int64(1)

	bf.Add(id, key)
	if !bf.Contains(id, key) {
		t.Error("False negative")
	}
}

func TestBloomFilterSharding(t *testing.T) {
	bf := NewSharedBloomFilter(10000, 0.05)

	// Write
	for i := 0; i < 1000; i++ {
		bf.Add(int64(i), []byte(fmt.Sprintf("k%d", i)))
	}

	// Read
	for i := 0; i < 1000; i++ {
		if !bf.Contains(int64(i), []byte(fmt.Sprintf("k%d", i))) {
			t.Errorf("Missing key %d", i)
		}
	}
}

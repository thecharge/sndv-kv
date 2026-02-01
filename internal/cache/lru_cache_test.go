package cache

import (
	"testing"
)

func TestLruCache_AllOps(t *testing.T) {
	c := NewLruCache(2)

	// Insert
	c.InsertIntoCache("k1", []byte("v1"))
	val, ok := c.RetrieveFromCache("k1")
	if !ok || string(val) != "v1" {
		t.Error("Insert/Retrieve failed")
	}

	// Update
	c.InsertIntoCache("k1", []byte("v1_updated"))
	val, _ = c.RetrieveFromCache("k1")
	if string(val) != "v1_updated" {
		t.Error("Update failed")
	}

	// Eviction
	c.InsertIntoCache("k2", []byte("v2"))
	c.InsertIntoCache("k3", []byte("v3")) // Should evict k1 (LRU) since we accessed it last before k2? No, we updated k1. k1 is MRU.
	// Wait, InsertIntoCache moves to front.
	// Sequence:
	// 1. Insert k1 (Front: k1)
	// 2. Update k1 (Front: k1)
	// 3. Insert k2 (Front: k2, k1)
	// 4. Insert k3 (Front: k3, k2). k1 evicted.

	if _, ok := c.RetrieveFromCache("k1"); ok {
		t.Error("k1 should be evicted")
	}
	if _, ok := c.RetrieveFromCache("k2"); !ok {
		t.Error("k2 should exist")
	}

	// Remove
	c.RemoveFromCache("k2")
	if _, ok := c.RetrieveFromCache("k2"); ok {
		t.Error("k2 should be removed")
	}

	// Retrieve Missing
	if _, ok := c.RetrieveFromCache("missing"); ok {
		t.Error("Found missing key")
	}
}

func TestLruCache_EdgeCases(t *testing.T) {
	c := NewLruCache(1)
	c.RemoveFromCache("missing") // Should not panic

	c.InsertIntoCache("k1", []byte("v1"))
	c.InsertIntoCache("k2", []byte("v2")) // Trigger eviction with capacity 1

	if _, ok := c.RetrieveFromCache("k1"); ok {
		t.Error("k1 should be evicted")
	}
}

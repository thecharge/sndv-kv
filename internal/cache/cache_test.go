package cache

import (
	"testing"
)

func TestCache(t *testing.T) {
	// Capacity 2
	c := New(2)

	c.Put("k1", []byte("v1"))
	c.Put("k2", []byte("v2"))

	if val, ok := c.Get("k1"); !ok || string(val) != "v1" {
		t.Error("Failed to get k1")
	}

	// Add k3, should evict k2 (because k1 was just used)
	c.Put("k3", []byte("v3"))

	if _, ok := c.Get("k2"); ok {
		t.Error("k2 should have been evicted")
	}
	if _, ok := c.Get("k1"); !ok {
		t.Error("k1 should still exist")
	}
}

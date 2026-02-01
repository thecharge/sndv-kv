package cache

import (
	"testing"
)

func TestLruCache_InsertAndRetrieve(t *testing.T) {
	cache := NewLruCache(2)

	cache.InsertIntoCache("key1", []byte("val1"))
	val, found := cache.RetrieveFromCache("key1")

	if !found {
		t.Error("Expected key1 to be found")
	}
	if string(val) != "val1" {
		t.Errorf("Expected val1, got %s", val)
	}
}

func TestLruCache_UpdateExisting(t *testing.T) {
	cache := NewLruCache(2)

	cache.InsertIntoCache("key1", []byte("val1"))
	cache.InsertIntoCache("key1", []byte("val1-updated"))

	val, _ := cache.RetrieveFromCache("key1")
	if string(val) != "val1-updated" {
		t.Errorf("Expected updated value, got %s", val)
	}
}

func TestLruCache_Eviction(t *testing.T) {
	cache := NewLruCache(2)

	cache.InsertIntoCache("key1", []byte("val1"))
	cache.InsertIntoCache("key2", []byte("val2"))
	// Access key1 to make key2 LRU
	cache.RetrieveFromCache("key1")

	// Insert key3, should evict key2
	cache.InsertIntoCache("key3", []byte("val3"))

	if _, found := cache.RetrieveFromCache("key2"); found {
		t.Error("Expected key2 to be evicted")
	}
	if _, found := cache.RetrieveFromCache("key1"); !found {
		t.Error("Expected key1 to remain")
	}
}

func TestLruCache_Remove(t *testing.T) {
	cache := NewLruCache(2)
	cache.InsertIntoCache("key1", []byte("val1"))

	cache.RemoveFromCache("key1")

	if _, found := cache.RetrieveFromCache("key1"); found {
		t.Error("Expected key1 to be removed")
	}

	// Remove non-existent safe check
	cache.RemoveFromCache("key-missing")
}

func TestLruCache_RetrieveMissing(t *testing.T) {
	cache := NewLruCache(2)
	if _, found := cache.RetrieveFromCache("missing"); found {
		t.Error("Expected false for missing key")
	}
}

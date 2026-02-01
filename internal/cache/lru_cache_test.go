package cache

import (
	"testing"
)

func TestLruCacheOperations(t *testing.T) {
	// 1. Initialize
	capacity := 2
	cache := NewLruCache(capacity)

	// 2. Insert Data
	key1 := "key1"
	val1 := []byte("value1")
	cache.Insert(key1, val1)

	// 3. Retrieve Data
	retrievedVal, found := cache.Retrieve(key1)
	if !found {
		t.Errorf("Failed to retrieve inserted key: %s", key1)
	}
	if string(retrievedVal) != string(val1) {
		t.Errorf("Value mismatch. Expected %s, got %s", val1, retrievedVal)
	}

	// 4. Test Eviction
	cache.Insert("key2", []byte("value2"))
	cache.Insert("key3", []byte("value3")) // Should evict key1 (LRU)

	_, foundKey1 := cache.Retrieve(key1)
	if foundKey1 {
		t.Error("Key1 should have been evicted")
	}

	_, foundKey3 := cache.Retrieve("key3")
	if !foundKey3 {
		t.Error("Key3 should be present")
	}
}

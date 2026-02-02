package storage

import (
	"sndv-kv/internal/common"
	"sync"
	"sync/atomic"
)

// MemoryTable is a simple in-memory key-value store
// Uses a single map with RWMutex for thread safety
type MemoryTable struct {
	data      map[string]common.Entry
	mutex     sync.RWMutex
	totalSize int64
}

// NewMemoryTable creates a new MemoryTable with the given capacity hint
func NewMemoryTable(capacity int) *MemoryTable {
	return &MemoryTable{
		data: make(map[string]common.Entry, capacity),
	}
}

// Put adds or updates a key-value pair
func (mt *MemoryTable) Put(key string, value []byte, expiry int64, isDeleted bool) {
	entrySize := int64(len(key) + len(value) + 16)

	mt.mutex.Lock()
	defer mt.mutex.Unlock()

	// Subtract old entry size if exists
	if old, exists := mt.data[key]; exists {
		atomic.AddInt64(&mt.totalSize, -(int64(len(old.Key) + len(old.Value) + 16)))
	}

	// Add new entry
	mt.data[key] = common.Entry{
		Key:             key,
		Value:           value,
		ExpiryTimestamp: expiry,
		IsDeleted:       isDeleted,
	}

	// Add new entry size
	atomic.AddInt64(&mt.totalSize, entrySize)
}

// Get retrieves a value by key
func (mt *MemoryTable) Get(key string) (common.Entry, bool) {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()

	val, ok := mt.data[key]
	return val, ok
}

// GetAll returns all entries (used for flushing)
func (mt *MemoryTable) GetAll() []common.Entry {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()

	entries := make([]common.Entry, 0, len(mt.data))
	for _, e := range mt.data {
		entries = append(entries, e)
	}
	return entries
}

// DumpToSlice appends all entries to the provided slice
// This avoids allocation if the slice has capacity
func (mt *MemoryTable) DumpToSlice(out []common.Entry) []common.Entry {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()

	for _, e := range mt.data {
		out = append(out, e)
	}
	return out
}

// Size returns the approximate size in bytes
func (mt *MemoryTable) Size() int64 {
	return atomic.LoadInt64(&mt.totalSize)
}

// Legacy compatibility: keep ShardedMemoryTable name as alias
type ShardedMemoryTable = MemoryTable
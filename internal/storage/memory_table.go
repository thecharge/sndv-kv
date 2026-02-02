package storage

import (
	"hash/fnv"
	"sndv-kv/internal/common"
	"sync"
	"sync/atomic"
)

const numShards = 32

// MemoryShard is a single shard with its own lock
type MemoryShard struct {
	data  map[string]common.Entry
	mutex sync.RWMutex
	size  atomic.Int64
}

// ShardedMemoryTable splits data across multiple shards to reduce lock contention
type ShardedMemoryTable struct {
	shards [numShards]*MemoryShard
}

// NewMemoryTable creates a new sharded memory table
func NewMemoryTable(capacity int) *ShardedMemoryTable {
	mt := &ShardedMemoryTable{}
	shardCap := capacity / numShards
	if shardCap < 1 {
		shardCap = 1
	}

	for i := 0; i < numShards; i++ {
		mt.shards[i] = &MemoryShard{
			data: make(map[string]common.Entry, shardCap),
		}
	}
	return mt
}

// getShardID returns the shard index for a key
func (mt *ShardedMemoryTable) getShardID(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % numShards)
}

// Put adds or updates a key-value pair
func (mt *ShardedMemoryTable) Put(key string, value []byte, expiry int64, isDeleted bool) {
	shardID := mt.getShardID(key)
	shard := mt.shards[shardID]

	entrySize := int64(len(key) + len(value) + 16)

	shard.mutex.Lock()
	defer shard.mutex.Unlock()

	// Subtract old entry size if exists
	if old, exists := shard.data[key]; exists {
		oldSize := int64(len(old.Key) + len(old.Value) + 16)
		shard.size.Add(-oldSize)
	}

	// Add new entry
	shard.data[key] = common.Entry{
		Key:             key,
		Value:           value,
		ExpiryTimestamp: expiry,
		IsDeleted:       isDeleted,
	}

	// Add new entry size
	shard.size.Add(entrySize)
}

// Get retrieves a value by key
func (mt *ShardedMemoryTable) Get(key string) (common.Entry, bool) {
	shardID := mt.getShardID(key)
	shard := mt.shards[shardID]

	shard.mutex.RLock()
	defer shard.mutex.RUnlock()

	val, ok := shard.data[key]
	return val, ok
}

// GetAll returns all entries (used for flushing)
func (mt *ShardedMemoryTable) GetAll() []common.Entry {
	var entries []common.Entry
	return mt.DumpToSlice(entries)
}

// DumpToSlice appends all entries to the provided slice
func (mt *ShardedMemoryTable) DumpToSlice(out []common.Entry) []common.Entry {
	for i := 0; i < numShards; i++ {
		shard := mt.shards[i]
		shard.mutex.RLock()
		for _, e := range shard.data {
			out = append(out, e)
		}
		shard.mutex.RUnlock()
	}
	return out
}

// Size returns the approximate total size in bytes
func (mt *ShardedMemoryTable) Size() int64 {
	var total int64
	for i := 0; i < numShards; i++ {
		total += mt.shards[i].size.Load()
	}
	return total
}

// Legacy type alias for compatibility
type MemoryTable = ShardedMemoryTable

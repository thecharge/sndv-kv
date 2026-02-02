package storage

import (
	"hash/fnv"
	"sndv-kv/internal/common"
	"sync"
	"sync/atomic"
)

const memTableShards = 32

type memoryShard struct {
	data  map[string]common.Entry
	mutex sync.RWMutex
}

type ShardedMemoryTable struct {
	shards    [memTableShards]*memoryShard
	totalSize int64
}

func NewMemoryTable(capacity int) *ShardedMemoryTable {
	t := &ShardedMemoryTable{}
	shardCap := capacity / memTableShards
	if shardCap < 1 {
		shardCap = 1
	}

	for i := 0; i < memTableShards; i++ {
		t.shards[i] = &memoryShard{
			data: make(map[string]common.Entry, shardCap),
		}
	}
	return t
}

func (mt *ShardedMemoryTable) getShard(key string) *memoryShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return mt.shards[h.Sum32()%memTableShards]
}

func (mt *ShardedMemoryTable) Put(key string, value []byte, expiry int64, isDeleted bool) {
	shard := mt.getShard(key)
	entrySize := int64(len(key) + len(value) + 16)

	shard.mutex.Lock()
	if old, exists := shard.data[key]; exists {
		atomic.AddInt64(&mt.totalSize, -(int64(len(old.Key) + len(old.Value) + 16)))
	}
	shard.data[key] = common.Entry{
		Key:             key,
		Value:           value,
		ExpiryTimestamp: expiry,
		IsDeleted:       isDeleted,
	}
	shard.mutex.Unlock()

	atomic.AddInt64(&mt.totalSize, entrySize)
}

func (mt *ShardedMemoryTable) Get(key string) (common.Entry, bool) {
	shard := mt.getShard(key)
	shard.mutex.RLock()
	defer shard.mutex.RUnlock()
	val, ok := shard.data[key]
	return val, ok
}

func (mt *ShardedMemoryTable) GetAll() []common.Entry {
	// Fallback that allocates
	var entries []common.Entry
	return mt.DumpToSlice(entries)
}

// DumpToSlice appends all entries to the provided slice, avoiding allocation
func (mt *ShardedMemoryTable) DumpToSlice(out []common.Entry) []common.Entry {
	for i := 0; i < memTableShards; i++ {
		shard := mt.shards[i]
		shard.mutex.RLock()
		for _, e := range shard.data {
			out = append(out, e)
		}
		shard.mutex.RUnlock()
	}
	return out
}

func (mt *ShardedMemoryTable) Size() int64 {
	return atomic.LoadInt64(&mt.totalSize)
}

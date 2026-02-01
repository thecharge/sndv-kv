package core

import (
	"sndv-kv/internal/cache"
	"sndv-kv/internal/common"
	"sndv-kv/internal/config"
	"sndv-kv/internal/storage"
	"sync"
)

type SystemState struct {
	Configuration config.SystemConfiguration

	// Interfaces used for abstraction
	MemTable     common.KeyValueStore
	ImmutableMem []common.KeyValueStore

	ActiveWal  common.WriteAheadLog
	FrozenWALs []common.WriteAheadLog

	SSTables    [][]storage.SSTableMetadata
	BloomFilter common.BloomFilter

	Mutex          sync.RWMutex
	FlushCondition *sync.Cond // RENAMED: Was FlushCond

	KeyCache *cache.LruCache
}

func NewSystemState(cfg config.SystemConfiguration) *SystemState {
	state := &SystemState{
		Configuration: cfg,
		MemTable:      storage.NewMemoryTable(int(cfg.MaximumMemtableSizeInBytes / 100)),
		SSTables:      make([][]storage.SSTableMetadata, 4),
		KeyCache:      cache.NewLruCache(cfg.KeyCacheCapacityCount),
		BloomFilter:   storage.NewSharedBloomFilter(10_000_000, cfg.BloomFilterFalsePositiveRate),
	}
	state.FlushCondition = sync.NewCond(&state.Mutex)
	return state
}

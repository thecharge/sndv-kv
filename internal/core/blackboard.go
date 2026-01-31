package core

import (
	"sndv-kv/internal/cache"
	"sndv-kv/internal/config"
	"sndv-kv/internal/storage"
	"sync"
)

type Blackboard struct {
	Config       config.Config
	MemTable     *storage.SwissTable
	ImmutableMem []*storage.SwissTable
	ActiveWal    *storage.WAL
	FrozenWALs   []*storage.WAL
	SSTables     [][]storage.SSTableMetadata
	SharedBloom  *storage.SharedBloom
	Mu           sync.RWMutex
	FlushCond    *sync.Cond
	KeyCache     *cache.Cache
}

func NewBlackboard(cfg config.Config) *Blackboard {
	bb := &Blackboard{
		Config:      cfg,
		MemTable:    storage.NewSwissTable(int(cfg.MaxMemTableSize / 100)),
		SSTables:    make([][]storage.SSTableMetadata, 4),
		KeyCache:    cache.New(cfg.KeyCacheSize),
		SharedBloom: storage.NewSharedBloom(10_000_000, cfg.BloomFPR),
	}
	bb.FlushCond = sync.NewCond(&bb.Mu)
	return bb
}

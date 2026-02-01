package core

import (
	"sndv-kv/internal/config"
	"testing"
)

func TestSystemStateInitialization(t *testing.T) {
	cfg := config.SystemConfiguration{
		MaximumMemtableSizeInBytes:   1024,
		BloomFilterFalsePositiveRate: 0.01,
		KeyCacheCapacityCount:        100,
	}

	state := NewSystemState(cfg)

	if state.MemTable == nil {
		t.Error("MemTable not initialized")
	}
	if state.BloomFilter == nil {
		t.Error("BloomFilter not initialized")
	}
	if state.SSTables == nil {
		t.Error("SSTables slice not initialized")
	}
}

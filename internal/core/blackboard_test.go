package core

import (
	"sndv-kv/internal/config"
	"testing"
)

func TestBlackboardInit(t *testing.T) {
	cfg := config.Config{
		MaxMemTableSize: 1024,
		KeyCacheSize:    500,
	}

	bb := NewBlackboard(cfg)

	if bb.MemTable == nil {
		t.Error("MemTable not initialized")
	}
	if bb.KeyCache == nil {
		t.Error("KeyCache not initialized")
	}
	if bb.FlushCond == nil {
		t.Error("FlushCond not initialized")
	}
}

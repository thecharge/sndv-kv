package storage

import (
	"fmt"
	"hash/crc32"
	"math"
	"sync"
)

const bloomShards = 32

type bloomShard struct {
	bitset []uint64
	mu     sync.RWMutex
}

type SharedBloom struct {
	shards []*bloomShard
	k      uint64
	shardSize uint64
}

func NewSharedBloom(n int, p float64) *SharedBloom {
	if n <= 0 { n = 1000 }
	if p <= 0 { p = 0.01 }

	ln2 := math.Log(2)
	nFloat := float64(n)
	mFloat := -(nFloat * math.Log(p)) / (ln2 * ln2)
	kFloat := (mFloat / nFloat) * ln2

	m := uint64(math.Ceil(mFloat))
	k := uint64(math.Ceil(kFloat))

	if k < 1 { k = 1 }
	if k > 30 { k = 30 }
	
	if m < 64 { m = 64 }
	if m > 16*1024*1024*1024 { m = 16 * 1024 * 1024 * 1024 }

	// Calculate size per shard
	sSize := (m + uint64(bloomShards) - 1) / uint64(bloomShards)

	shards := make([]*bloomShard, bloomShards)
	for i := 0; i < bloomShards; i++ {
		shards[i] = &bloomShard{
			bitset: make([]uint64, (sSize+63)/64),
		}
	}

	return &SharedBloom{
		shards:    shards,
		k:         k,
		shardSize: sSize,
	}
}

func (sb *SharedBloom) Add(fileID int64, key []byte) {
	shardIdx := fileID % bloomShards
	shard := sb.shards[shardIdx]

	prefix := fmt.Sprintf("%d:", fileID)
	h1 := uint64(crc32.ChecksumIEEE(append([]byte(prefix), key...)))
	h2 := h1 >> 16

	shard.mu.Lock()
	defer shard.mu.Unlock()

	for i := uint64(0); i < sb.k; i++ {
		idx := (h1 + i*h2) % sb.shardSize
		shard.bitset[idx/64] |= (1 << (idx % 64))
	}
}

func (sb *SharedBloom) MayContain(fileID int64, key []byte) bool {
	shardIdx := fileID % bloomShards
	shard := sb.shards[shardIdx]

	prefix := fmt.Sprintf("%d:", fileID)
	h1 := uint64(crc32.ChecksumIEEE(append([]byte(prefix), key...)))
	h2 := h1 >> 16

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	for i := uint64(0); i < sb.k; i++ {
		idx := (h1 + i*h2) % sb.shardSize
		if shard.bitset[idx/64]&(1<<(idx%64)) == 0 {
			return false
		}
	}
	return true
}
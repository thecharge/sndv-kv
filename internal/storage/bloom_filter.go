package storage

import (
	"fmt"
	"hash/crc32"
	"math"
	"sync"
)

const bloomShardCount = 32

type bloomShard struct {
	data  []uint64
	mutex sync.RWMutex
}

type SharedBloomFilter struct {
	shards    []*bloomShard
	hashCount uint64
	shardSize uint64
}

func NewSharedBloomFilter(expectedItems int, falsePositiveRate float64) *SharedBloomFilter {
	if expectedItems <= 0 {
		expectedItems = 1000
	}
	if falsePositiveRate <= 0 {
		falsePositiveRate = 0.01
	}

	ln2 := math.Log(2)
	n := float64(expectedItems)
	m := -(n * math.Log(falsePositiveRate)) / (ln2 * ln2)
	k := (m / n) * ln2

	bits := uint64(math.Ceil(m))
	hashes := uint64(math.Ceil(k))

	if bits < 64 {
		bits = 64
	}
	if bits > 16*1024*1024*1024 {
		bits = 16 * 1024 * 1024 * 1024
	}

	bitsPerShard := (bits + uint64(bloomShardCount) - 1) / uint64(bloomShardCount)

	shards := make([]*bloomShard, bloomShardCount)
	for i := 0; i < bloomShardCount; i++ {
		shards[i] = &bloomShard{data: make([]uint64, (bitsPerShard+63)/64)}
	}

	return &SharedBloomFilter{shards: shards, hashCount: hashes, shardSize: bitsPerShard}
}

func (bf *SharedBloomFilter) Add(id int64, key []byte) {
	shardIdx := id % bloomShardCount
	shard := bf.shards[shardIdx]
	prefix := fmt.Sprintf("%d:", id)
	h1 := uint64(crc32.ChecksumIEEE(append([]byte(prefix), key...)))
	h2 := h1 >> 16

	shard.mutex.Lock()
	defer shard.mutex.Unlock()
	for i := uint64(0); i < bf.hashCount; i++ {
		idx := (h1 + i*h2) % bf.shardSize
		shard.data[idx/64] |= (1 << (idx % 64))
	}
}

func (bf *SharedBloomFilter) Contains(id int64, key []byte) bool {
	shardIdx := id % bloomShardCount
	shard := bf.shards[shardIdx]
	prefix := fmt.Sprintf("%d:", id)
	h1 := uint64(crc32.ChecksumIEEE(append([]byte(prefix), key...)))
	h2 := h1 >> 16

	shard.mutex.RLock()
	defer shard.mutex.RUnlock()
	for i := uint64(0); i < bf.hashCount; i++ {
		idx := (h1 + i*h2) % bf.shardSize
		if shard.data[idx/64]&(1<<(idx%64)) == 0 {
			return false
		}
	}
	return true
}

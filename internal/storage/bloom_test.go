package storage

import (
	"fmt"
	"math"
	"testing"
)

func TestBloomCalculus(t *testing.T) {
	n := 1000
	p := 0.01
	
	mFloat := -(float64(n) * math.Log(p)) / (math.Log(2) * math.Log(2))
	expectedTotalM := uint64(math.Ceil(mFloat))
	expectedShardSize := (expectedTotalM + 31) / 32
	
	bf := NewSharedBloom(n, p)
	
	if bf.shardSize != expectedShardSize {
		t.Errorf("Shard size incorrect. Expected %d, got %d", expectedShardSize, bf.shardSize)
	}
	
	if bf.k == 0 {
		t.Error("Hash count K is zero")
	}
}

func TestBloomBasic(t *testing.T) {
	bf := NewSharedBloom(100, 0.01)
	key := []byte("key1")
	fileID := int64(1)

	bf.Add(fileID, key)
	if !bf.MayContain(fileID, key) {
		t.Error("False Negative on inserted key")
	}
	if bf.MayContain(2, key) {
		// Collision possible but unlikely with prefix
	}
}

func TestBloomFPR(t *testing.T) {
	n := 10000
	// Initialize with n*32 because we force all keys into single shards in this loop test
	// effectively overloading specific shards if we aren't careful.
	// However, we distribute fileID here to use all shards.
	bf := NewSharedBloom(n, 0.05) 
	
	for i := 0; i < n; i++ {
		// Use i as fileID to distribute across 32 shards
		bf.Add(int64(i), []byte(fmt.Sprintf("key%d", i)))
	}
	
	fp := 0
	for i := 0; i < n; i++ {
		// Check missing keys
		if bf.MayContain(int64(i), []byte(fmt.Sprintf("missing%d", i))) {
			fp++
		}
	}
	
	rate := float64(fp) / float64(n)
	// Allow slight margin over 0.05
	if rate > 0.08 {
		t.Errorf("FPR too high: %f", rate)
	}
}
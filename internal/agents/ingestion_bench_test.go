package agents

import (
	"fmt"
	"sndv-kv/internal/config"
	"sndv-kv/internal/storage"
	testFactory "sndv-kv/internal/testing"
	"testing"
)

func BenchmarkSingleIngestion(b *testing.B) {
	f := testFactory.NewTestFactory(&testing.T{})
	defer f.Cleanup()

	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 64 * 1024 * 1024
		c.EnableDiskDurability = false
	})
	InitializeIngestionSubsystem(state)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		val := []byte("testvalue1234567890")
		_ = SubmitIngestionRequest(key, val, 0, false)
	}
}

func BenchmarkBatchIngestion(b *testing.B) {
	f := testFactory.NewTestFactory(&testing.T{})
	defer f.Cleanup()

	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 64 * 1024 * 1024
		c.EnableDiskDurability = false
	})
	InitializeIngestionSubsystem(state)

	batchSize := 100
	keys := make([]string, batchSize)
	vals := make([][]byte, batchSize)
	ttls := make([]int, batchSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < batchSize; j++ {
			keys[j] = fmt.Sprintf("key%d_%d", i, j)
			vals[j] = []byte("testvalue1234567890")
			ttls[j] = 0
		}
		_ = SubmitBatchIngestion(keys, vals, ttls)
	}
}

func BenchmarkConcurrentIngestion(b *testing.B) {
	f := testFactory.NewTestFactory(&testing.T{})
	defer f.Cleanup()

	state := f.CreateSystem(func(c *config.SystemConfiguration) {
		c.MaximumMemtableSizeInBytes = 64 * 1024 * 1024
		c.EnableDiskDurability = false
	})
	InitializeIngestionSubsystem(state)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i)
			val := []byte("testvalue1234567890")
			_ = SubmitIngestionRequest(key, val, 0, false)
			i++
		}
	})
}

func BenchmarkMemTablePut(b *testing.B) {
	mt := storage.NewMemoryTable(1000000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i)
			val := []byte("testvalue1234567890")
			mt.Put(key, val, 0, false)
			i++
		}
	})
}

func BenchmarkMemTableGet(b *testing.B) {
	mt := storage.NewMemoryTable(1000000)

	// Pre-populate
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i)
		val := []byte("testvalue1234567890")
		mt.Put(key, val, 0, false)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%10000)
			mt.Get(key)
			i++
		}
	})
}

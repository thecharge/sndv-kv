package internal

import (
	"os"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"strconv"
	"sync/atomic"
	"testing"
)

func BenchmarkEngineWriteParallel(b *testing.B) {
	cfg := config.Config{
		DataDir:         "./bench_data_go",
		WalPath:         "./bench_data_go/wal.log",
		MaxMemTableSize: 64 * 1024 * 1024,
		Durability:      false,
		KeyCacheSize:    10000,
	}
	os.RemoveAll(cfg.DataDir)
	os.MkdirAll(cfg.DataDir, 0755)
	defer os.RemoveAll(cfg.DataDir)

	bb := core.NewBlackboard(cfg)
	agents.InitIngest(bb)
	agents.StartFlushAgent(bb)

	var counter int64
	val := []byte("x")

	b.ResetTimer()
	b.SetParallelism(20)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&counter, 1)
			key := strconv.FormatInt(i, 10)
			if err := agents.SubmitIngest(key, val, 0, false); err != nil {
				b.Fatal(err)
			}
		}
	})
}

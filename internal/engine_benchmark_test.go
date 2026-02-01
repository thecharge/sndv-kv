package internal

import (
	"os"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"strconv"
	"sync/atomic"
	"testing"
)

func BenchmarkEngineWriteParallel(b *testing.B) {
	dir := "./bench_data"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	logger.InitializeLogger(dir, "ERROR")

	cfg := config.SystemConfiguration{
		DataDirectoryPath:          dir,
		WriteAheadLogFilePath:      dir + "/wal.log",
		MaximumMemtableSizeInBytes: 64 * 1024 * 1024,
		EnableDiskDurability:       false,
		KeyCacheCapacityCount:      10000,
		MaximumCpuCount:            8,
	}

	bb := core.NewSystemState(cfg)
	agents.InitializeIngestionSubsystem(bb)
	agents.StartFlushAgentInBackground(bb)

	var counter int64
	val := []byte("val")

	b.ResetTimer()
	b.SetParallelism(20)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&counter, 1)
			key := strconv.FormatInt(i, 10)
			if err := agents.SubmitIngestionRequest(key, val, 0, false); err != nil {
				b.Fatal(err)
			}
		}
	})
}

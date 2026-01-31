package metrics

import (
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

// GlobalMetrics holds the atomic counters for the system.
type GlobalMetrics struct {
	WriteOps      int64
	ReadOps       int64
	CacheHits     int64
	MemTableBytes int64
	WALSizeBytes  int64
	WALSyncs      int64
	SysMemAlloc   uint64
	Goroutines    int
}

var Global GlobalMetrics

func IncRead()                { atomic.AddInt64(&Global.ReadOps, 1) }
func IncCacheHit()            { atomic.AddInt64(&Global.CacheHits, 1) }
func IncWALSync()             { atomic.AddInt64(&Global.WALSyncs, 1) }
func SetMemUsage(bytes int64) { atomic.StoreInt64(&Global.MemTableBytes, bytes) }

// StartSystemMonitor updates low-level system metrics in the background.
func StartSystemMonitor(dataDir, walPath string) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for range ticker.C {
			// 1. Memory Stats
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			atomic.StoreUint64(&Global.SysMemAlloc, m.Alloc)
			Global.Goroutines = runtime.NumGoroutine()

			// 2. WAL Size
			if info, err := os.Stat(walPath); err == nil {
				atomic.StoreInt64(&Global.WALSizeBytes, info.Size())
			}

			// 3. Data Dir Size (Optional, heavy on I/O, simplified here)
			// We skip full dir walk every 2s to avoid stealing IOPS from the engine.
		}
	}()
}

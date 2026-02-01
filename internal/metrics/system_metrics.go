package metrics

import (
	"sync/atomic"
)

type SystemMetricsRegistry struct {
	WriteOperationsCount int64 `json:"write_operations_count"`
	ReadOperationsCount  int64 `json:"read_operations_count"`
	CacheHitCount        int64 `json:"cache_hit_count"`
	CacheMissCount       int64 `json:"cache_miss_count"`
	// Exported as WriteOps for compatibility with agent logic
	WriteOps int64 `json:"-"`
}

var Global SystemMetricsRegistry

func IncrementCacheHitCount() {
	atomic.AddInt64(&Global.CacheHitCount, 1)
}

func IncrementReadOperationsCount() {
	atomic.AddInt64(&Global.ReadOperationsCount, 1)
}

func IncrementCacheMissCount() {
	atomic.AddInt64(&Global.CacheMissCount, 1)
}

// GetCurrentState returns a snapshot for the API
func GetCurrentState() map[string]int64 {
	return map[string]int64{
		"write_ops":  atomic.LoadInt64(&Global.WriteOps),
		"read_ops":   atomic.LoadInt64(&Global.ReadOperationsCount),
		"cache_hits": atomic.LoadInt64(&Global.CacheHitCount),
	}
}

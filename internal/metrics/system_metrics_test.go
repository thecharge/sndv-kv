package metrics

import (
	"testing"
)

func TestMetricsCounters(t *testing.T) {
	// Reset
	Global = SystemMetricsRegistry{}

	IncrementReadOperationsCount()
	if Global.ReadOperationsCount != 1 {
		t.Errorf("Expected 1 read op, got %d", Global.ReadOperationsCount)
	}

	IncrementCacheHitCount()
	if Global.CacheHitCount != 1 {
		t.Errorf("Expected 1 cache hit, got %d", Global.CacheHitCount)
	}

	IncrementCacheMissCount()
	if Global.CacheMissCount != 1 {
		t.Errorf("Expected 1 cache miss, got %d", Global.CacheMissCount)
	}

	snapshot := GetCurrentState()
	if snapshot["read_ops"] != 1 {
		t.Error("Snapshot failed to reflect read ops")
	}
}

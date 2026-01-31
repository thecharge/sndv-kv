package metrics

import (
	"testing"
)

func TestMetricsInc(t *testing.T) {
	initial := Global.ReadOps
	IncRead()
	if Global.ReadOps != initial+1 {
		t.Error("IncRead failed")
	}

	SetMemUsage(1024)
	if Global.MemTableBytes != 1024 {
		t.Error("SetMemUsage failed")
	}
}

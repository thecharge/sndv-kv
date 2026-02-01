package agents

import (
	"container/heap"
	"fmt"
	"os"
	"sndv-kv/internal/common"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	"time"
)

type MergeItem struct {
	Entry    common.Entry
	SourceID int
}
type MergeHeap []*MergeItem

func (h MergeHeap) Len() int            { return len(h) }
func (h MergeHeap) Less(i, j int) bool  { return h[i].Entry.Key < h[j].Entry.Key }
func (h MergeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *MergeHeap) Push(x interface{}) { *h = append(*h, x.(*MergeItem)) }
func (h *MergeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

func StartCompactionAgentInBackground(bb *core.SystemState) {
	go func() {
		interval := time.Duration(bb.Configuration.CompactionIntervalInSeconds) * time.Second
		if interval == 0 {
			interval = 5 * time.Second
		}
		ticker := time.NewTicker(interval)

		for range ticker.C {
			checkAndRunCompaction(bb)
		}
	}()
}

func checkAndRunCompaction(bb *core.SystemState) {
	bb.Mutex.Lock()
	if len(bb.SSTables) == 0 {
		bb.Mutex.Unlock()
		return
	}
	l0Count := len(bb.SSTables[0])
	trigger := bb.Configuration.LevelZeroCompactionTriggerCount

	if l0Count < trigger {
		bb.Mutex.Unlock()
		return
	}

	// Capture tables
	tables := make([]storage.SSTableMetadata, l0Count)
	copy(tables, bb.SSTables[0])
	bb.SSTables[0] = make([]storage.SSTableMetadata, 0)
	bb.Mutex.Unlock()

	executeCompaction(bb, tables)
}

func executeCompaction(bb *core.SystemState, tables []storage.SSTableMetadata) {
	logger.LogInfoEvent("Compacting %d L0 tables", len(tables))

	mergedFile, newMeta, err := performMerge(tables, bb.Configuration.DataDirectoryPath, bb.BloomFilter)

	bb.Mutex.Lock()
	defer bb.Mutex.Unlock()

	if err != nil {
		logger.LogErrorEvent("Compaction Failed: %v", err)
		bb.SSTables[0] = append(tables, bb.SSTables[0]...)
		return
	}

	commitCompaction(bb, tables, newMeta, mergedFile)
}

func commitCompaction(bb *core.SystemState, oldTables []storage.SSTableMetadata, newMeta storage.SSTableMetadata, filename string) {
	if len(bb.SSTables) < 2 {
		bb.SSTables = append(bb.SSTables, make([]storage.SSTableMetadata, 0))
	}
	bb.SSTables[1] = append(bb.SSTables[1], newMeta)

	for _, t := range oldTables {
		os.Remove(t.Filename)
	}
	logger.LogInfoEvent("Compaction Success: %s", filename)
}

func performMerge(tables []storage.SSTableMetadata, dir string, bloom common.BloomFilter) (string, storage.SSTableMetadata, error) {
	iters, err := createIterators(tables)
	if err != nil {
		return "", storage.SSTableMetadata{}, err
	}
	defer closeIterators(iters)

	entries := mergeIterators(iters)

	fname := fmt.Sprintf("%s/L1_%d.sst", dir, time.Now().UnixNano())
	meta, err := storage.WriteSortedStringTableToDisk(entries, fname, 1, bloom)
	return fname, meta, err
}

func createIterators(tables []storage.SSTableMetadata) ([]*storage.SSTableReader, error) {
	iters := make([]*storage.SSTableReader, 0, len(tables))
	for _, meta := range tables {
		iter, err := storage.NewSSTableReader(meta.Filename)
		if err != nil {
			closeIterators(iters)
			return nil, err
		}
		iters = append(iters, iter)
	}
	return iters, nil
}

func closeIterators(iters []*storage.SSTableReader) {
	for _, it := range iters {
		it.Close()
	}
}

func mergeIterators(iters []*storage.SSTableReader) []common.Entry {
	mh := &MergeHeap{}
	heap.Init(mh)

	for i, iter := range iters {
		if e, ok := iter.Next(); ok {
			heap.Push(mh, &MergeItem{Entry: e, SourceID: i})
		}
	}

	var entries []common.Entry
	var lastKey string

	for mh.Len() > 0 {
		top := heap.Pop(mh).(*MergeItem)

		if len(entries) > 0 && lastKey == top.Entry.Key {
			entries[len(entries)-1] = top.Entry
		} else {
			entries = append(entries, top.Entry)
			lastKey = top.Entry.Key
		}

		if e, ok := iters[top.SourceID].Next(); ok {
			heap.Push(mh, &MergeItem{Entry: e, SourceID: top.SourceID})
		}
	}
	return entries
}

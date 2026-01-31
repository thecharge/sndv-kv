package agents

import (
	"container/heap"
	"fmt"
	"os"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	"time"
)

type MergeItem struct {
	Entry    storage.Entry
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

func StartCompactionAgent(bb *core.Blackboard) {
	go func() {
		interval := time.Duration(bb.Config.CompactionInterval) * time.Second
		if interval == 0 {
			interval = 5 * time.Second
		}
		ticker := time.NewTicker(interval)

		for range ticker.C {
			bb.Mu.Lock()
			if len(bb.SSTables) == 0 {
				bb.Mu.Unlock()
				continue
			}
			l0Count := len(bb.SSTables[0])
			if l0Count >= bb.Config.L0CompactionTrigger {
				logger.Info("Compaction: %d L0 Tables", l0Count)
				tables := make([]storage.SSTableMetadata, l0Count)
				copy(tables, bb.SSTables[0])
				bb.SSTables[0] = make([]storage.SSTableMetadata, 0)
				bb.Mu.Unlock()

				mergedFilename, newMeta, err := performStreamingMerge(tables, bb.Config.DataDir, bb.SharedBloom)
				bb.Mu.Lock()
				if err == nil {
					if len(bb.SSTables) < 2 {
						bb.SSTables = append(bb.SSTables, make([]storage.SSTableMetadata, 0))
					}
					bb.SSTables[1] = append(bb.SSTables[1], newMeta)
					for _, t := range tables {
						os.Remove(t.Filename)
					}
					logger.Info("Compaction Success: %s", mergedFilename)
				} else {
					logger.Error("Compaction Failed: %v", err)
					bb.SSTables[0] = append(tables, bb.SSTables[0]...)
				}
			}
			bb.Mu.Unlock()
		}
	}()
}

func performStreamingMerge(tables []storage.SSTableMetadata, dataDir string, bloom *storage.SharedBloom) (string, storage.SSTableMetadata, error) {
	iters := make([]*storage.SSTableIterator, 0, len(tables))
	mh := &MergeHeap{}
	heap.Init(mh)

	for i, meta := range tables {
		iter, err := storage.NewIterator(meta.Filename)
		if err != nil {
			return "", storage.SSTableMetadata{}, err
		}
		iters = append(iters, iter)
		if iter.Next() {
			heap.Push(mh, &MergeItem{Entry: iter.Current, SourceID: i})
		}
	}

	var entries []storage.Entry
	var lastKey string
	for mh.Len() > 0 {
		top := heap.Pop(mh).(*MergeItem)
		if len(entries) > 0 && lastKey == top.Entry.Key {
			entries[len(entries)-1] = top.Entry
		} else {
			entries = append(entries, top.Entry)
			lastKey = top.Entry.Key
		}
		if iters[top.SourceID].Next() {
			heap.Push(mh, &MergeItem{Entry: iters[top.SourceID].Current, SourceID: top.SourceID})
		}
	}
	for _, it := range iters {
		it.Close()
	}

	fname := fmt.Sprintf("%s/L1_%d.sst", dataDir, time.Now().UnixNano())
	meta, err := storage.WriteSSTable(entries, fname, 1, bloom)
	return fname, meta, err
}

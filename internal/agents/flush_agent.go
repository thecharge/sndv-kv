package agents

import (
	"fmt"
	"sndv-kv/internal/common"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	"sort"
	"sync"
	"time"
)

var flushBufferPool = sync.Pool{
	New: func() interface{} {
		// Pre-allocate 10k items for flush buffer
		s := make([]common.Entry, 0, 10000)
		return &s
	},
}

func StartFlushAgentInBackground(bb *core.SystemState) {
	go func() {
		for {
			table := waitForFlush(bb)
			if table != nil {
				processFlush(bb, table)
			}
		}
	}()
}

func waitForFlush(bb *core.SystemState) common.KeyValueStore {
	bb.Mutex.Lock()
	defer bb.Mutex.Unlock()

	for len(bb.ImmutableMem) == 0 {
		bb.FlushCondition.Wait()
	}
	return bb.ImmutableMem[0]
}

func processFlush(bb *core.SystemState, table common.KeyValueStore) {
	filename := fmt.Sprintf("%s/L0_%d.sst", bb.Configuration.DataDirectoryPath, time.Now().UnixNano())

	// MEMORY OPTIMIZATION: Get buffer from pool
	bufPtr := flushBufferPool.Get().(*[]common.Entry)
	entries := (*bufPtr)[:0] // Reset length

	// Dump MemTable into buffer
	if mem, ok := table.(*storage.ShardedMemoryTable); ok {
		// Optimized path avoiding intermediate allocs
		entries = mem.DumpToSlice(entries)
	} else {
		// Fallback for tests
		entries = table.GetAll()
	}

	// SSTables MUST be sorted
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	meta, err := storage.WriteSortedStringTableToDisk(entries, filename, 0, bb.BloomFilter)

	// Return buffer to pool
	flushBufferPool.Put(bufPtr)

	commitFlush(bb, meta, err, filename, len(entries))
}

func commitFlush(bb *core.SystemState, meta storage.SSTableMetadata, err error, filename string, count int) {
	bb.Mutex.Lock()
	defer bb.Mutex.Unlock()

	if err != nil {
		logger.LogErrorEvent("Flush Error: %v", err)
		return
	}

	if len(bb.SSTables) == 0 {
		bb.SSTables = make([][]storage.SSTableMetadata, 4)
	}
	bb.SSTables[0] = append(bb.SSTables[0], meta)

	if len(bb.ImmutableMem) > 0 {
		bb.ImmutableMem = bb.ImmutableMem[1:]
	}

	rotateFrozenWal(bb)
	logger.LogInfoEvent("Flushed %d keys to %s", count, filename)
}

func rotateFrozenWal(bb *core.SystemState) {
	if !bb.Configuration.EnableDiskDurability {
		return
	}
	if len(bb.FrozenWALs) > 0 {
		bb.FrozenWALs[0].Delete()
		bb.FrozenWALs = bb.FrozenWALs[1:]
	}
}

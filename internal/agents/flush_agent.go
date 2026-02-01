package agents

import (
	"fmt"
	"sndv-kv/internal/common"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	"time"
)

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
		bb.FlushCondition.Wait() // FIX: Updated name
	}
	return bb.ImmutableMem[0]
}

func processFlush(bb *core.SystemState, table common.KeyValueStore) {
	filename := fmt.Sprintf("%s/L0_%d.sst", bb.Configuration.DataDirectoryPath, time.Now().UnixNano())
	entries := table.GetAll()

	meta, err := storage.WriteSortedStringTableToDisk(entries, filename, 0, bb.BloomFilter)

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

package agents

import (
	"fmt"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/storage"
	"time"
)

func StartFlushAgent(bb *core.Blackboard) {
	go func() {
		for {
			bb.Mu.Lock()
			for len(bb.ImmutableMem) == 0 {
				bb.FlushCond.Wait()
			}
			tableToFlush := bb.ImmutableMem[0]
			bb.Mu.Unlock()

			if tableToFlush != nil {
				filename := fmt.Sprintf("%s/L0_%d.sst", bb.Config.DataDir, time.Now().UnixNano())
				entries := tableToFlush.AllEntries()
				meta, err := storage.WriteSSTable(entries, filename, 0, bb.SharedBloom)

				bb.Mu.Lock()
				if err == nil {
					if len(bb.SSTables) == 0 {
						bb.SSTables = make([][]storage.SSTableMetadata, 4)
					}
					bb.SSTables[0] = append(bb.SSTables[0], meta)
					if len(bb.ImmutableMem) > 0 {
						bb.ImmutableMem = bb.ImmutableMem[1:]
					}
					if bb.Config.Durability && len(bb.FrozenWALs) > 0 {
						bb.FrozenWALs[0].Delete()
						bb.FrozenWALs = bb.FrozenWALs[1:]
					}
					logger.Info("Flushed %d keys to %s", len(entries), filename)
				} else {
					logger.Error("Flush Error: %v", err)
				}
				bb.Mu.Unlock()
			}
		}
	}()
}

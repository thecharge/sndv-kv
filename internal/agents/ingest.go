package agents

import (
	"fmt"
	"hash/fnv"
	"runtime"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/metrics"
	"sndv-kv/internal/storage"
	"sync"
	"time"
)

type IngestReq struct {
	Key     string
	Val     []byte
	TTL     int
	Deleted bool
	Resp    chan error
}

var (
	shardQueues  []chan IngestReq
	numShards    int
	respChanPool = sync.Pool{
		New: func() interface{} { return make(chan error, 1) },
	}
)

func InitIngest(bb *core.Blackboard) {
	numShards = runtime.NumCPU()
	if bb.Config.MaxCPU > 0 {
		numShards = bb.Config.MaxCPU
	}
	
	shardQueues = make([]chan IngestReq, numShards)
	for i := 0; i < numShards; i++ {
		shardQueues[i] = make(chan IngestReq, 10000)
		go runShard(i, shardQueues[i], bb)
	}
	logger.Info("Ingest: %d Shards", numShards)
}

func SubmitIngest(key string, val []byte, ttl int, deleted bool) error {
	h := fnv.New32a()
	h.Write([]byte(key))
	shardID := int(h.Sum32()) % numShards

	respChan := respChanPool.Get().(chan error)
	shardQueues[shardID] <- IngestReq{Key: key, Val: val, TTL: ttl, Deleted: deleted, Resp: respChan}
	
	err := <-respChan
	respChanPool.Put(respChan)
	return err
}

func runShard(id int, queue chan IngestReq, bb *core.Blackboard) {
	batch := make([]IngestReq, 0, 1000)
	walEntries := make([]storage.Entry, 0, 1000)
	
	flush := func() {
		count := len(batch)
		if count == 0 { return }
		walEntries = walEntries[:0]
		now := time.Now()
		
		for i := 0; i < count; i++ {
			var exp int64
			if batch[i].TTL > 0 {
				exp = now.Add(time.Duration(batch[i].TTL) * time.Second).UnixNano()
			}
			walEntries = append(walEntries, storage.Entry{
				Key:       batch[i].Key,
				Value:     batch[i].Val,
				ExpiresAt: exp,
				Deleted:   batch[i].Deleted,
			})
		}

		if bb.Config.Durability && bb.ActiveWal != nil {
			if err := bb.ActiveWal.AppendBatch(walEntries); err != nil {
				logger.Error("WAL Error: %v", err)
				for _, req := range batch { req.Resp <- err }
				batch = batch[:0]
				return
			}
		}

		bb.Mu.Lock()
		for i := 0; i < count; i++ {
			bb.MemTable.Put(batch[i].Key, batch[i].Val, walEntries[i].ExpiresAt, batch[i].Deleted)
		}
		memSize := bb.MemTable.SizeBytes
		
		if memSize >= bb.Config.MaxMemTableSize {
			rotateMemTable(bb)
		}
		bb.Mu.Unlock()

		metrics.Global.WriteOps += int64(count)
		for _, req := range batch { req.Resp <- nil }
		batch = batch[:0]
	}

	for {
		req := <-queue
		batch = append(batch, req)
		n := len(queue)
		if n > 1999 { n = 1999 }
		for i := 0; i < n; i++ {
			batch = append(batch, <-queue)
		}
		flush()
	}
}

func rotateMemTable(bb *core.Blackboard) {
	logger.Info("Rotating MemTable...")
	bb.ImmutableMem = append(bb.ImmutableMem, bb.MemTable)
	bb.MemTable = storage.NewSwissTable(1024 * 1024)
	
	if bb.Config.Durability && bb.ActiveWal != nil {
		newPath := fmt.Sprintf("%s.%d", bb.Config.WalPath, time.Now().UnixNano())
		nw, err := storage.OpenWAL(newPath, bb.Config.Durability)
		
		if err != nil {
			logger.Error("WAL Rotate Failed: %v - continuing with old WAL", err)
		} else {
			bb.FrozenWALs = append(bb.FrozenWALs, bb.ActiveWal)
			bb.ActiveWal = nw
		}
	}
	bb.FlushCond.Signal()
}

func SubmitBatch(keys []string, vals [][]byte, ttls []int) error {
	for i := range keys {
		if err := SubmitIngest(keys[i], vals[i], ttls[i], false); err != nil {
			return err
		}
	}
	return nil
}
package agents

import (
	"fmt"
	"hash/fnv"
	"runtime"
	"sndv-kv/internal/common"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/metrics"
	"sndv-kv/internal/storage"
	"sync"
	"time"
)

type IngestReq struct {
	Key             string
	Val             []byte
	TTL             int
	IsDeleted       bool
	ResponseChannel chan error
}

var (
	shardQueues  []chan IngestReq
	numShards    int
	respChanPool = sync.Pool{
		New: func() interface{} { return make(chan error, 1) },
	}
)

func InitializeIngestionSubsystem(bb *core.SystemState) {
	numShards = runtime.NumCPU()
	if bb.Configuration.MaximumCpuCount > 0 {
		numShards = bb.Configuration.MaximumCpuCount
	}
	
	shardQueues = make([]chan IngestReq, numShards)
	for i := 0; i < numShards; i++ {
		shardQueues[i] = make(chan IngestReq, 10000)
		go runShard(i, shardQueues[i], bb)
	}
	logger.LogInfoEvent("Ingest initialized with %d shards", numShards)
}

func SubmitIngestionRequest(key string, val []byte, ttl int, deleted bool) error {
	h := fnv.New32a()
	h.Write([]byte(key))
	shardID := int(h.Sum32()) % numShards

	respChan := respChanPool.Get().(chan error)
	shardQueues[shardID] <- IngestReq{
		Key:             key, 
		Val:             val, 
		TTL:             ttl, 
		IsDeleted:       deleted, 
		ResponseChannel: respChan,
	}
	
	err := <-respChan
	respChanPool.Put(respChan)
	return err
}

func SubmitBatchIngestion(keys []string, vals [][]byte, ttls []int) error {
    // Group by shard
    shardBatches := make(map[int][]IngestReq)
    
    for i := range keys {
        h := fnv.New32a()
        h.Write([]byte(keys[i]))
        shardID := int(h.Sum32()) % numShards
        
        respChan := respChanPool.Get().(chan error)
        shardBatches[shardID] = append(shardBatches[shardID], IngestReq{
            Key: keys[i],
            Val: vals[i],
            TTL: ttls[i],
            ResponseChannel: respChan,
        })
    }
    
    // Submit all batches
    var wg sync.WaitGroup
    errors := make(chan error, len(shardBatches))
    
    for shardID, batch := range shardBatches {
        wg.Add(1)
        go func(id int, reqs []IngestReq) {
            defer wg.Done()
            for _, req := range reqs {
                shardQueues[id] <- req
                if err := <-req.ResponseChannel; err != nil {
                    errors <- err
                    return
                }
                respChanPool.Put(req.ResponseChannel)
            }
        }(shardID, batch)
    }
    
    wg.Wait()
    close(errors)
    
    for err := range errors {
        if err != nil {
            return err
        }
    }
    return nil
}

func runShard(id int, queue chan IngestReq, bb *core.SystemState) {
	batch := make([]IngestReq, 0, 1000)
	
	for {
		req := <-queue
		batch = append(batch, req)
		drainQueue(queue, &batch)
		processBatch(id, batch, bb)
		batch = batch[:0]
	}
}

func drainQueue(queue chan IngestReq, batch *[]IngestReq) {
	limit := 1999
	count := len(queue)
	if count > limit {
		count = limit
	}
	for i := 0; i < count; i++ {
		*batch = append(*batch, <-queue)
	}
}

func processBatch(shardID int, batch []IngestReq, bb *core.SystemState) {
	if len(batch) == 0 {
		return
	}

	entries := prepareEntries(batch)

	if writeWalIfEnabled(shardID, batch, entries, bb) != nil {
		return
	}

	applyToMemTable(bb, batch, entries)
	
	metrics.Global.WriteOps += int64(len(batch))
	
	// Notify callers
	for _, req := range batch {
		req.ResponseChannel <- nil
	}
}

func prepareEntries(batch []IngestReq) []common.Entry {
	entries := make([]common.Entry, len(batch))
	now := time.Now()
	
	for i, req := range batch {
		var exp int64
		if req.TTL > 0 {
			exp = now.Add(time.Duration(req.TTL) * time.Second).UnixNano()
		}
		entries[i] = common.Entry{
			Key:             req.Key,
			Value:           req.Val,
			ExpiryTimestamp: exp,
			IsDeleted:       req.IsDeleted,
		}
	}
	return entries
}

func writeWalIfEnabled(shardID int, batch []IngestReq, entries []common.Entry, bb *core.SystemState) error {
	if !bb.Configuration.EnableDiskDurability || bb.ActiveWal == nil {
		return nil
	}

	if err := bb.ActiveWal.WriteBatch(entries); err != nil {
		logger.LogErrorEvent("Shard %d WAL Error: %v", shardID, err)
		for _, req := range batch {
			req.ResponseChannel <- err
		}
		return err
	}
	return nil
}

func applyToMemTable(bb *core.SystemState, batch []IngestReq, entries []common.Entry) {
	bb.Mutex.Lock()
	defer bb.Mutex.Unlock()

	for i := 0; i < len(batch); i++ {
		bb.MemTable.Put(
			batch[i].Key, 
			batch[i].Val, 
			entries[i].ExpiryTimestamp, 
			batch[i].IsDeleted,
		)
	}
	
	if bb.MemTable.Size() >= bb.Configuration.MaximumMemtableSizeInBytes {
		rotateMemTable(bb)
	}
}

func rotateMemTable(bb *core.SystemState) {
	logger.LogInfoEvent("Rotating MemTable...")
	bb.ImmutableMem = append(bb.ImmutableMem, bb.MemTable)
	bb.MemTable = storage.NewMemoryTable(1024 * 1024)
	
	if bb.Configuration.EnableDiskDurability && bb.ActiveWal != nil {
		rotateWal(bb)
	}
	bb.FlushCondition.Signal()
}

func rotateWal(bb *core.SystemState) {
	newPath := fmt.Sprintf("%s.%d", bb.Configuration.WriteAheadLogFilePath, time.Now().UnixNano())
	nw, err := storage.NewDiskWAL(newPath, bb.Configuration.EnableDiskDurability)
	
	if err != nil {
		logger.LogErrorEvent("WAL Rotate Failed: %v", err)
		return
	}
	bb.FrozenWALs = append(bb.FrozenWALs, bb.ActiveWal)
	bb.ActiveWal = nw
}
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

type BatchIngestReq struct {
	Items           []IngestReq
	ResponseChannel chan error
}

var (
	shardQueues  []chan interface{}
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

	shardQueues = make([]chan interface{}, numShards)
	for i := 0; i < numShards; i++ {
		shardQueues[i] = make(chan interface{}, 10000)
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
		Key: key, Val: val, TTL: ttl, IsDeleted: deleted, ResponseChannel: respChan,
	}

	err := <-respChan
	respChanPool.Put(respChan)
	return err
}

func SubmitBatchIngestion(keys []string, vals [][]byte, ttls []int) error {
	if len(keys) == 0 {
		return nil
	}

	// 1. Group keys by shard
	shardBatches := make(map[int][]IngestReq)
	for i := range keys {
		h := fnv.New32a()
		h.Write([]byte(keys[i]))
		shardID := int(h.Sum32()) % numShards

		shardBatches[shardID] = append(shardBatches[shardID], IngestReq{
			Key: keys[i], Val: vals[i], TTL: ttls[i], IsDeleted: false,
		})
	}

	// 2. Dispatch grouped batches
	activeShards := len(shardBatches)
	responseChan := make(chan error, activeShards)

	for id, items := range shardBatches {
		shardQueues[id] <- BatchIngestReq{
			Items: items, ResponseChannel: responseChan,
		}
	}

	// 3. Wait for all shards
	var finalErr error
	for i := 0; i < activeShards; i++ {
		if err := <-responseChan; err != nil && finalErr == nil {
			finalErr = err
		}
	}
	return finalErr
}

func runShard(id int, queue chan interface{}, bb *core.SystemState) {
	singleBuffer := make([]IngestReq, 0, 1000)

	for payload := range queue {
		switch req := payload.(type) {
		case IngestReq:
			singleBuffer = append(singleBuffer, req)
			drainQueue(queue, &singleBuffer)
			processBatch(id, singleBuffer, bb)
			singleBuffer = singleBuffer[:0]

		case BatchIngestReq:
			processBatch(id, req.Items, bb)
			req.ResponseChannel <- nil
		}
	}
}

func drainQueue(queue chan interface{}, batch *[]IngestReq) {
	for i := 0; i < 100; i++ { // Limit drain to prevent starvation
		select {
		case payload := <-queue:
			if req, ok := payload.(IngestReq); ok {
				*batch = append(*batch, req)
			} else {
				// Put back complex batch (simplified strategy)
				// Realistically, mixed load is rare in bench
				go func() { queue <- payload }()
				return
			}
		default:
			return
		}
	}
}

func processBatch(shardID int, batch []IngestReq, bb *core.SystemState) {
	entries := prepareEntries(batch)

	if err := writeWalIfEnabled(shardID, entries, bb); err != nil {
		notifyErrors(batch, err)
		return
	}

	applyToMemTable(bb, batch, entries)
	metrics.Global.WriteOps += int64(len(batch))
	notifySuccess(batch)
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
			Key: req.Key, Value: req.Val, ExpiryTimestamp: exp, IsDeleted: req.IsDeleted,
		}
	}
	return entries
}

func writeWalIfEnabled(shardID int, entries []common.Entry, bb *core.SystemState) error {
	if !bb.Configuration.EnableDiskDurability || bb.ActiveWal == nil {
		return nil
	}
	if err := bb.ActiveWal.WriteBatch(entries); err != nil {
		logger.LogErrorEvent("WAL Error Shard %d: %v", shardID, err)
		return err
	}
	return nil
}

func applyToMemTable(bb *core.SystemState, batch []IngestReq, entries []common.Entry) {
	for i := 0; i < len(batch); i++ {
		bb.MemTable.Put(batch[i].Key, batch[i].Val, entries[i].ExpiryTimestamp, batch[i].IsDeleted)
	}

	if bb.MemTable.Size() >= bb.Configuration.MaximumMemtableSizeInBytes {
		bb.Mutex.Lock()
		if bb.MemTable.Size() >= bb.Configuration.MaximumMemtableSizeInBytes {
			rotateMemTable(bb)
		}
		bb.Mutex.Unlock()
	}
}

func notifySuccess(batch []IngestReq) {
	for _, req := range batch {
		if req.ResponseChannel != nil {
			req.ResponseChannel <- nil
		}
	}
}

func notifyErrors(batch []IngestReq, err error) {
	for _, req := range batch {
		if req.ResponseChannel != nil {
			req.ResponseChannel <- err
		}
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
	if nw, err := storage.NewDiskWAL(newPath, true); err == nil {
		bb.FrozenWALs = append(bb.FrozenWALs, bb.ActiveWal)
		bb.ActiveWal = nw
	} else {
		logger.LogErrorEvent("WAL Rotate Failed: %v", err)
	}
}

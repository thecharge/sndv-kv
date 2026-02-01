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

type ShardChannels struct {
	SingleQueue chan *IngestReq
	BatchQueue  chan *BatchIngestReq
}

var (
	shardChannels []ShardChannels
	numShards     int

	reqPool = sync.Pool{
		New: func() interface{} { return &IngestReq{} },
	}
	respChanPool = sync.Pool{
		New: func() interface{} { return make(chan error, 1) },
	}
)

func InitializeIngestionSubsystem(bb *core.SystemState) {
	numShards = runtime.NumCPU()
	if bb.Configuration.MaximumCpuCount > 0 {
		numShards = bb.Configuration.MaximumCpuCount
	}

	shardChannels = make([]ShardChannels, numShards)
	for i := 0; i < numShards; i++ {
		shardChannels[i] = ShardChannels{
			SingleQueue: make(chan *IngestReq, 10000),
			BatchQueue:  make(chan *BatchIngestReq, 100),
		}
		go runShard(i, shardChannels[i], bb)
	}
	logger.LogInfoEvent("Ingest initialized with %d shards", numShards)
}

func SubmitIngestionRequest(key string, val []byte, ttl int, deleted bool) error {
	h := fnv.New32a()
	h.Write([]byte(key))
	shardID := int(h.Sum32()) % numShards

	req := reqPool.Get().(*IngestReq)
	req.Key = key
	req.Val = val
	req.TTL = ttl
	req.IsDeleted = deleted

	respChan := respChanPool.Get().(chan error)
	req.ResponseChannel = respChan

	shardChannels[shardID].SingleQueue <- req

	err := <-respChan

	respChanPool.Put(respChan)
	req.Val = nil
	req.Key = ""
	reqPool.Put(req)

	return err
}

func SubmitBatchIngestion(keys []string, vals [][]byte, ttls []int) error {
	if len(keys) == 0 {
		return nil
	}

	shardBatches := groupItemsByShard(keys, vals, ttls)
	return dispatchAndAwaitBatches(shardBatches)
}

func groupItemsByShard(keys []string, vals [][]byte, ttls []int) map[int][]IngestReq {
	batches := make(map[int][]IngestReq)
	for i := range keys {
		h := fnv.New32a()
		h.Write([]byte(keys[i]))
		shardID := int(h.Sum32()) % numShards

		batches[shardID] = append(batches[shardID], IngestReq{
			Key:       keys[i],
			Val:       vals[i],
			TTL:       ttls[i],
			IsDeleted: false,
		})
	}
	return batches
}

func dispatchAndAwaitBatches(batches map[int][]IngestReq) error {
	activeShards := len(batches)
	responseChan := make(chan error, activeShards)

	for id, items := range batches {
		req := &BatchIngestReq{
			Items:           items,
			ResponseChannel: responseChan,
		}
		shardChannels[id].BatchQueue <- req
	}

	var finalErr error
	for i := 0; i < activeShards; i++ {
		err := <-responseChan
		if err != nil && finalErr == nil {
			finalErr = err
		}
	}
	return finalErr
}

func runShard(id int, chans ShardChannels, bb *core.SystemState) {
	itemBuffer := make([]IngestReq, 0, 1000)

	for {
		select {
		case req := <-chans.SingleQueue:
			itemBuffer = append(itemBuffer, *req)
			drainSingleQueue(chans.SingleQueue, &itemBuffer)
			processBatch(id, itemBuffer, bb)
			itemBuffer = itemBuffer[:0]

		case batch := <-chans.BatchQueue:
			processBatch(id, batch.Items, bb)
			batch.ResponseChannel <- nil
		}
	}
}

func drainSingleQueue(queue chan *IngestReq, batch *[]IngestReq) {
	count := len(queue)
	if count > 100 {
		count = 100
	}
	// Deterministic read based on length snapshot.
	// Safe because we are the only consumer.
	for i := 0; i < count; i++ {
		req := <-queue
		*batch = append(*batch, *req)
	}
}

func processBatch(shardID int, batch []IngestReq, bb *core.SystemState) {
	if len(batch) == 0 {
		return
	}

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
		entries[i] = createEntry(req, now)
	}
	return entries
}

func createEntry(req IngestReq, now time.Time) common.Entry {
	var exp int64
	if req.TTL > 0 {
		exp = now.Add(time.Duration(req.TTL) * time.Second).UnixNano()
	}
	return common.Entry{
		Key:             req.Key,
		Value:           req.Val,
		ExpiryTimestamp: exp,
		IsDeleted:       req.IsDeleted,
	}
}

func writeWalIfEnabled(shardID int, entries []common.Entry, bb *core.SystemState) error {
	if !bb.Configuration.EnableDiskDurability || bb.ActiveWal == nil {
		return nil
	}
	if err := bb.ActiveWal.WriteBatch(entries); err != nil {
		logger.LogErrorEvent("Shard %d WAL Error: %v", shardID, err)
		return err
	}
	return nil
}

func applyToMemTable(bb *core.SystemState, batch []IngestReq, entries []common.Entry) {
	for i := 0; i < len(batch); i++ {
		bb.MemTable.Put(batch[i].Key, batch[i].Val, entries[i].ExpiryTimestamp, batch[i].IsDeleted)
		if bb.KeyCache != nil {
			bb.KeyCache.RemoveFromCache(batch[i].Key)
		}
	}

	if bb.MemTable.Size() >= bb.Configuration.MaximumMemtableSizeInBytes {
		checkAndRotate(bb)
	}
}

func checkAndRotate(bb *core.SystemState) {
	bb.Mutex.Lock()
	defer bb.Mutex.Unlock()

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
	nw, err := storage.NewDiskWAL(newPath, true)

	if err != nil {
		logger.LogErrorEvent("WAL Rotate Failed: %v", err)
		return
	}

	bb.FrozenWALs = append(bb.FrozenWALs, bb.ActiveWal)
	bb.ActiveWal = nw
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

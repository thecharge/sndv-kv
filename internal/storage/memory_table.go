package storage

import (
	"sndv-kv/internal/common"
	"sync"
)

type MemoryTable struct {
	data        map[string]common.Entry
	mutex       sync.RWMutex
	sizeInBytes int64
}

func NewMemoryTable(capacity int) *MemoryTable {
	return &MemoryTable{
		data: make(map[string]common.Entry, capacity),
	}
}

func (mt *MemoryTable) Put(key string, value []byte, expiry int64, isDeleted bool) {
	mt.mutex.Lock()
	defer mt.mutex.Unlock()

	newSize := int64(len(key) + len(value) + 16)
	if old, exists := mt.data[key]; exists {
		oldSize := int64(len(old.Key) + len(old.Value) + 16)
		mt.sizeInBytes -= oldSize
	}

	mt.data[key] = common.Entry{
		Key:             key,
		Value:           value,
		ExpiryTimestamp: expiry,
		IsDeleted:       isDeleted,
	}
	mt.sizeInBytes += newSize
}

func (mt *MemoryTable) Get(key string) (common.Entry, bool) {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()
	e, ok := mt.data[key]
	return e, ok
}

func (mt *MemoryTable) GetAll() []common.Entry {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()
	res := make([]common.Entry, 0, len(mt.data))
	for _, e := range mt.data {
		res = append(res, e)
	}
	return res
}

func (mt *MemoryTable) Size() int64 {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()
	return mt.sizeInBytes
}

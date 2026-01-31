package storage

import "sync"

// used for the memtable
type Entry struct {
	Key       string
	Value     []byte
	ExpiresAt int64
	Deleted   bool
}

type SwissTable struct {
	data      map[string]Entry
	mu        sync.RWMutex
	SizeBytes int64
}

func NewSwissTable(cap int) *SwissTable {
	return &SwissTable{
		data: make(map[string]Entry, cap),
	}
}

func (st *SwissTable) Put(key string, val []byte, exp int64, del bool) {
	st.mu.Lock()
	defer st.mu.Unlock()

	sizeDiff := int64(len(key) + len(val) + 16)
	if old, ok := st.data[key]; ok {
		sizeDiff -= int64(len(old.Key) + len(old.Value) + 16)
	}

	st.data[key] = Entry{Key: key, Value: val, ExpiresAt: exp, Deleted: del}
	st.SizeBytes += sizeDiff
}

func (st *SwissTable) Get(key string) (Entry, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	e, ok := st.data[key]
	return e, ok
}

func (st *SwissTable) AllEntries() []Entry {
	st.mu.RLock()
	defer st.mu.RUnlock()
	res := make([]Entry, 0, len(st.data))
	for _, e := range st.data {
		res = append(res, e)
	}
	return res
}

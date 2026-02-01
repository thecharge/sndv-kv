package common

type Entry struct {
	Key             string
	Value           []byte
	ExpiryTimestamp int64
	IsDeleted       bool
}

type BloomFilter interface {
	Add(id int64, key []byte)
	Contains(id int64, key []byte) bool
}

type WriteAheadLog interface {
	WriteBatch(entries []Entry) error
	Replay(callback func(Entry)) error
	Close() error
	Delete() error
}

type KeyValueStore interface {
	Put(key string, value []byte, expiry int64, isDeleted bool)
	Get(key string) (Entry, bool)
	GetAll() []Entry
	Size() int64
}

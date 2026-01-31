package cache

import (
	"container/list"
	"sync"
)

type Cache struct {
	Capacity int
	ll       *list.List
	items    map[string]*list.Element
	mu       sync.Mutex
}

type entry struct {
	key   string
	value []byte
}

func New(capacity int) *Cache {
	return &Cache{
		Capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
	}
}

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, hit := c.items[key]; hit {
		c.ll.MoveToFront(elem)
		return elem.Value.(*entry).value, true
	}
	return nil, false
}

func (c *Cache) Put(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing
	if elem, hit := c.items[key]; hit {
		c.ll.MoveToFront(elem)
		elem.Value.(*entry).value = value
		return
	}

	// Add new
	ele := c.ll.PushFront(&entry{key, value})
	c.items[key] = ele

	// Evict if full
	if c.ll.Len() > c.Capacity {
		oldest := c.ll.Back()
		if oldest != nil {
			c.ll.Remove(oldest)
			kv := oldest.Value.(*entry)
			delete(c.items, kv.key)
		}
	}
}

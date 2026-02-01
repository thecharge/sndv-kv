package cache

import (
	"container/list"
	"sync"
)

type LruCache struct {
	CapacityCount int
	evictionList  *list.List
	itemsMap      map[string]*list.Element
	mutex         sync.Mutex
}

type cacheEntry struct {
	key   string
	value []byte
}

func NewLruCache(capacity int) *LruCache {
	return &LruCache{
		CapacityCount: capacity,
		evictionList:  list.New(),
		itemsMap:      make(map[string]*list.Element),
	}
}

func (c *LruCache) Retrieve(key string) ([]byte, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, exists := c.itemsMap[key]; exists {
		c.evictionList.MoveToFront(element)
		return element.Value.(*cacheEntry).value, true
	}
	return nil, false
}

func (c *LruCache) Insert(key string, value []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, exists := c.itemsMap[key]; exists {
		c.evictionList.MoveToFront(element)
		element.Value.(*cacheEntry).value = value
		return
	}

	newElement := c.evictionList.PushFront(&cacheEntry{key, value})
	c.itemsMap[key] = newElement

	if c.evictionList.Len() > c.CapacityCount {
		oldestElement := c.evictionList.Back()
		if oldestElement != nil {
			c.evictionList.Remove(oldestElement)
			entry := oldestElement.Value.(*cacheEntry)
			delete(c.itemsMap, entry.key)
		}
	}
}

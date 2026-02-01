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

func (c *LruCache) RetrieveFromCache(key string) ([]byte, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	element, exists := c.itemsMap[key]
	if !exists {
		return nil, false
	}

	c.evictionList.MoveToFront(element)
	entry := element.Value.(*cacheEntry)
	return entry.value, true
}

func (c *LruCache) InsertIntoCache(key string, value []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, exists := c.itemsMap[key]; exists {
		c.updateExistingEntry(element, value)
		return
	}

	c.addNewEntry(key, value)
	c.enforceCapacity()
}

func (c *LruCache) RemoveFromCache(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, exists := c.itemsMap[key]; exists {
		c.evictionList.Remove(element)
		delete(c.itemsMap, key)
	}
}

func (c *LruCache) updateExistingEntry(element *list.Element, value []byte) {
	c.evictionList.MoveToFront(element)
	entry := element.Value.(*cacheEntry)
	entry.value = value
}

func (c *LruCache) addNewEntry(key string, value []byte) {
	newEntry := &cacheEntry{key, value}
	element := c.evictionList.PushFront(newEntry)
	c.itemsMap[key] = element
}

func (c *LruCache) enforceCapacity() {
	if c.evictionList.Len() <= c.CapacityCount {
		return
	}

	oldestElement := c.evictionList.Back()
	if oldestElement != nil {
		c.evictionList.Remove(oldestElement)
		entry := oldestElement.Value.(*cacheEntry)
		delete(c.itemsMap, entry.key)
	}
}

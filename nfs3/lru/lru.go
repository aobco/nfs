package lru

import (
	"container/list"
	"sync"
)

type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	lruList  *list.List
	mutex    sync.Mutex
}

type entry struct {
	key   string
	value []byte
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		lruList:  list.New(),
	}
}

func (c *LRUCache) Get(key string) ([]byte, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, ok := c.cache[key]; ok {
		c.lruList.MoveToFront(element)
		return element.Value.(*entry).value, true
	}

	return nil, false
}

func (c *LRUCache) Add(key string, value []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, ok := c.cache[key]; ok {
		c.lruList.MoveToFront(element)
		element.Value.(*entry).value = value
		return
	}

	if len(c.cache) >= c.capacity {
		back := c.lruList.Back()
		if back != nil {
			delete(c.cache, back.Value.(*entry).key)
			c.lruList.Remove(back)
		}
	}

	newElement := c.lruList.PushFront(&entry{key, value})
	c.cache[key] = newElement
}

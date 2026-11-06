package lru

import (
	"container/list"
	"sync"
)

type entry[K comparable, V any] struct {
	key   K
	value V
}

type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*list.Element
	order    *list.List
}

func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity <= 0 {
		capacity = 512
	}
	return &Cache[K, V]{
		capacity: capacity,
		items:    map[K]*list.Element{},
		order:    list.New(),
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(entry[K, V]).value, true
	}
	var zero V
	return zero, false
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setLocked(key, value)
}

func (c *Cache[K, V]) setLocked(key K, value V) {
	if elem, ok := c.items[key]; ok {
		elem.Value = entry[K, V]{key: key, value: value}
		c.order.MoveToFront(elem)
		return
	}
	elem := c.order.PushFront(entry[K, V]{key: key, value: value})
	c.items[key] = elem
	if c.order.Len() > c.capacity {
		last := c.order.Back()
		if last != nil {
			c.order.Remove(last)
			kv := last.Value.(entry[K, V])
			delete(c.items, kv.key)
		}
	}
}

// GetOrCreate coalesces misses. create must not reenter this cache.
func (c *Cache[K, V]) GetOrCreate(key K, create func() V) V {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.items[key]; ok {
		c.order.MoveToFront(element)
		return element.Value.(entry[K, V]).value
	}
	value := create()
	c.setLocked(key, value)
	return value
}

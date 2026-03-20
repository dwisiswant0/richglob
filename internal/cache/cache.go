package cache

import (
	"sync"
	"sync/atomic"
)

type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	entries  map[K]V
	order    []K
	next     int
	capacity int
	last     atomic.Pointer[entry[K, V]]
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

func New[K comparable, V any](capacity int) *Cache[K, V] {
	return &Cache[K, V]{
		entries:  make(map[K]V, capacity),
		order:    make([]K, 0, capacity),
		capacity: capacity,
	}
}

func (cache *Cache[K, V]) Get(key K) (V, bool) {
	if entry := cache.last.Load(); entry != nil && entry.key == key {
		return entry.value, true
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	value, ok := cache.entries[key]
	if ok {
		cache.last.Store(&entry[K, V]{key: key, value: value})
	}

	return value, ok
}

func (cache *Cache[K, V]) Add(key K, value V) {
	cache.mu.Lock()
	if _, ok := cache.entries[key]; ok {
		cache.mu.Unlock()

		return
	}

	if len(cache.order) < cache.capacity {
		cache.order = append(cache.order, key)
	} else {
		delete(cache.entries, cache.order[cache.next])
		cache.order[cache.next] = key
		cache.next = (cache.next + 1) % cache.capacity
	}

	cache.entries[key] = value
	cache.last.Store(&entry[K, V]{key: key, value: value})
	cache.mu.Unlock()
}

package cache

import (
	"sync"
	"time"
)

// Cache is the public interface. Implementations are private.
type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V, ttl time.Duration)
	Delete(key K)
}

// NewInMemory returns a Cache backed by a sync.RWMutex map.
func NewInMemory[K comparable, V any]() Cache[K, V] {
	return &inMemory[K, V]{items: make(map[K]entry[V])}
}

type inMemory[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]entry[V]
}

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

func (c *inMemory[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[key]
	if !ok || (!e.expiresAt.IsZero() && time.Now().After(e.expiresAt)) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *inMemory[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.items[key] = entry[V]{value: value, expiresAt: exp}
}

func (c *inMemory[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

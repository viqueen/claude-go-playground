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
	e, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		var zero V
		return zero, false
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		c.mu.RUnlock()
		c.mu.Lock()
		if e2, ok2 := c.items[key]; ok2 && !e2.expiresAt.IsZero() && time.Now().After(e2.expiresAt) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		var zero V
		return zero, false
	}
	value := e.value
	c.mu.RUnlock()
	return value, true
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

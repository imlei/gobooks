// 遵循project_guide.md
package cache

import (
	"sync"
	"time"
)

// entry stores a typed value together with its expiry deadline.
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTLCache is a thread-safe, in-memory key-value store with per-entry TTL.
// It is acceleration-only infrastructure: the authoritative data always lives
// in the database. Never treat a cache miss as a security boundary.
//
// Type parameters:
//
//	K — comparable key type (e.g. string)
//	V — value type (any pointer or struct)
type TTLCache[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]entry[V]
	ttl  time.Duration
	stop chan struct{}
}

// New creates a TTLCache with the given TTL and starts a background cleanup
// goroutine that evicts expired entries every 5 minutes.
// Call Close() when the cache is no longer needed (e.g. in tests) to stop it.
func New[K comparable, V any](ttl time.Duration) *TTLCache[K, V] {
	c := &TTLCache[K, V]{
		data: make(map[K]entry[V]),
		ttl:  ttl,
		stop: make(chan struct{}),
	}
	go c.cleanupLoop()
	return c
}

// Get returns the value for key and true if the entry exists and has not expired.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores value under key with the cache's configured TTL.
func (c *TTLCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.data[key] = entry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// Delete removes a single key from the cache immediately.
func (c *TTLCache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
}

// Flush removes all entries from the cache.
// Use this to invalidate after a write that affects many keys (e.g. company settings change).
func (c *TTLCache[K, V]) Flush() {
	c.mu.Lock()
	c.data = make(map[K]entry[V])
	c.mu.Unlock()
}

// Len returns the number of entries currently in the cache (including not-yet-evicted expired ones).
func (c *TTLCache[K, V]) Len() int {
	c.mu.RLock()
	n := len(c.data)
	c.mu.RUnlock()
	return n
}

// FlushWhere removes all entries for which match(key) returns true.
// It holds the write lock for the full scan — use sparingly on large caches.
func (c *TTLCache[K, V]) FlushWhere(match func(K) bool) {
	c.mu.Lock()
	for k := range c.data {
		if match(k) {
			delete(c.data, k)
		}
	}
	c.mu.Unlock()
}

// Close stops the background cleanup goroutine. Safe to call more than once.
func (c *TTLCache[K, V]) Close() {
	select {
	case <-c.stop:
		// already closed
	default:
		close(c.stop)
	}
}

func (c *TTLCache[K, V]) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stop:
			return
		}
	}
}

func (c *TTLCache[K, V]) evictExpired() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.data {
		if now.After(e.expiresAt) {
			delete(c.data, k)
		}
	}
	c.mu.Unlock()
}

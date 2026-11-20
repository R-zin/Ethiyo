// Package cache provides a small, dependency-free TTL cache with built-in
// request coalescing ("single flight") for expensive upstream loads.
package cache

import (
	"context"
	"sync"
	"time"
)

// LoadFunc produces a value for a cache key.
type LoadFunc[V any] func(ctx context.Context) (V, error)

type entry[V any] struct {
	val     V
	expires time.Time
}

type call[V any] struct {
	done chan struct{}
	val  V
	err  error
}

// TTL is a concurrency-safe, size-bounded cache whose entries expire a fixed
// duration after they are stored. Concurrent GetOrLoad calls for the same key
// share a single in-flight load. A non-positive TTL disables storage but keeps
// coalescing, so callers can turn caching off without losing the protection
// against thundering herds.
type TTL[K comparable, V any] struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	items      map[K]entry[V]
	inflight   map[K]*call[V]
	now        func() time.Time
}

// New creates a cache. maxEntries <= 0 defaults to 1024.
func New[K comparable, V any](ttl time.Duration, maxEntries int) *TTL[K, V] {
	if maxEntries <= 0 {
		maxEntries = 1024
	}
	return &TTL[K, V]{
		ttl:        ttl,
		maxEntries: maxEntries,
		items:      make(map[K]entry[V]),
		inflight:   make(map[K]*call[V]),
		now:        time.Now,
	}
}

// Get returns a fresh cached value.
func (c *TTL[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getLocked(key)
}

func (c *TTL[K, V]) getLocked(key K) (V, bool) {
	e, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	if !c.now().Before(e.expires) {
		delete(c.items, key)
		var zero V
		return zero, false
	}
	return e.val, true
}

// Set stores a value, evicting expired entries (and then the entry closest to
// expiry) when the cache is full.
func (c *TTL[K, V]) Set(key K, val V) {
	if c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setLocked(key, val)
}

func (c *TTL[K, V]) setLocked(key K, val V) {
	now := c.now()
	if _, exists := c.items[key]; !exists && len(c.items) >= c.maxEntries {
		c.evictLocked(now)
	}
	c.items[key] = entry[V]{val: val, expires: now.Add(c.ttl)}
}

func (c *TTL[K, V]) evictLocked(now time.Time) {
	var (
		victim     K
		victimExp  time.Time
		haveVictim bool
	)
	for k, e := range c.items {
		if !now.Before(e.expires) {
			delete(c.items, k)
			continue
		}
		if !haveVictim || e.expires.Before(victimExp) {
			victim, victimExp, haveVictim = k, e.expires, true
		}
	}
	if len(c.items) >= c.maxEntries && haveVictim {
		delete(c.items, victim)
	}
}

// Len returns the number of stored (possibly expired) entries.
func (c *TTL[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// GetOrLoad returns the cached value for key, or runs load to produce it.
// Concurrent callers for the same key share one load. hit reports whether the
// value came from the cache rather than from a load. Errors are never cached.
//
// The load runs detached from the caller's cancellation (context values are
// preserved), so one impatient caller cannot fail every other caller waiting
// on the same load; each caller still stops waiting when its own ctx ends.
// Loads must therefore bound their own runtime (e.g. via HTTP or browser
// timeouts).
func (c *TTL[K, V]) GetOrLoad(ctx context.Context, key K, load LoadFunc[V]) (val V, hit bool, err error) {
	c.mu.Lock()
	if v, ok := c.getLocked(key); ok {
		c.mu.Unlock()
		return v, true, nil
	}
	cl, running := c.inflight[key]
	if !running {
		cl = &call[V]{done: make(chan struct{})}
		c.inflight[key] = cl
		go c.run(context.WithoutCancel(ctx), key, cl, load)
	}
	c.mu.Unlock()

	select {
	case <-cl.done:
		return cl.val, false, cl.err
	case <-ctx.Done():
		var zero V
		return zero, false, ctx.Err()
	}
}

func (c *TTL[K, V]) run(ctx context.Context, key K, cl *call[V], load LoadFunc[V]) {
	defer func() {
		// A panicking load must not leave waiters blocked forever.
		if r := recover(); r != nil {
			cl.err = &PanicError{Value: r}
		}
		c.mu.Lock()
		if cl.err == nil && c.ttl > 0 {
			c.setLocked(key, cl.val)
		}
		delete(c.inflight, key)
		c.mu.Unlock()
		close(cl.done)
	}()
	cl.val, cl.err = load(ctx)
}

// PanicError wraps a value recovered from a panicking load.
type PanicError struct{ Value any }

func (e *PanicError) Error() string { return "cache: load panicked" }

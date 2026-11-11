package chalo

import (
	"sync"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
)

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

func (e cacheEntry[T]) expired(now time.Time) bool {
	return now.After(e.expiresAt)
}

// RouteCache is a thread-safe in-memory TTL cache for bus route and live tracking info.
type RouteCache struct {
	mu          sync.RWMutex
	routes      map[string]cacheEntry[*models.BusRouteInfo]
	tracking    map[string]cacheEntry[*models.BusTrackingInfo]
	routeTTL    time.Duration
	trackingTTL time.Duration
}

// NewRouteCache creates a new RouteCache with specified or default TTLs.
func NewRouteCache(routeTTL, trackingTTL time.Duration) *RouteCache {
	if routeTTL <= 0 {
		routeTTL = 15 * time.Minute
	}
	if trackingTTL <= 0 {
		trackingTTL = 10 * time.Second
	}
	return &RouteCache{
		routes:      make(map[string]cacheEntry[*models.BusRouteInfo]),
		tracking:    make(map[string]cacheEntry[*models.BusTrackingInfo]),
		routeTTL:    routeTTL,
		trackingTTL: trackingTTL,
	}
}

// GetRoute returns the cached BusRouteInfo for busCode if present and not expired.
func (c *RouteCache) GetRoute(busCode string) (*models.BusRouteInfo, bool) {
	c.mu.RLock()
	entry, exists := c.routes[busCode]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}
	if entry.expired(time.Now()) {
		c.mu.Lock()
		delete(c.routes, busCode)
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

// SetRoute stores the BusRouteInfo for busCode with the configured TTL (or optional override).
func (c *RouteCache) SetRoute(busCode string, route *models.BusRouteInfo, ttl ...time.Duration) {
	if route == nil {
		return
	}
	duration := c.routeTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		duration = ttl[0]
	}

	c.mu.Lock()
	c.routes[busCode] = cacheEntry[*models.BusRouteInfo]{
		value:     route,
		expiresAt: time.Now().Add(duration),
	}
	c.mu.Unlock()
}

// GetTracking returns the cached BusTrackingInfo for busCode if present and not expired.
func (c *RouteCache) GetTracking(busCode string) (*models.BusTrackingInfo, bool) {
	c.mu.RLock()
	entry, exists := c.tracking[busCode]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}
	if entry.expired(time.Now()) {
		c.mu.Lock()
		delete(c.tracking, busCode)
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

// SetTracking stores the BusTrackingInfo for busCode with the configured TTL (or optional override).
func (c *RouteCache) SetTracking(busCode string, tracking *models.BusTrackingInfo, ttl ...time.Duration) {
	if tracking == nil {
		return
	}
	duration := c.trackingTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		duration = ttl[0]
	}

	c.mu.Lock()
	c.tracking[busCode] = cacheEntry[*models.BusTrackingInfo]{
		value:     tracking,
		expiresAt: time.Now().Add(duration),
	}
	c.mu.Unlock()
}

// Clear flushes all cached entries.
func (c *RouteCache) Clear() {
	c.mu.Lock()
	c.routes = make(map[string]cacheEntry[*models.BusRouteInfo])
	c.tracking = make(map[string]cacheEntry[*models.BusTrackingInfo])
	c.mu.Unlock()
}

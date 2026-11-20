package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/gin-gonic/gin"
)

type clientBucket struct {
	tokens    float64
	lastCheck time.Time
}

const defaultMaxClients = 10000

// RateLimiter manages token bucket rate limits per client IP.
type RateLimiter struct {
	rate    float64 // tokens per second
	burst   float64 // max bucket capacity
	clients map[string]*clientBucket
	mu      sync.Mutex

	// ttl is how long an idle client bucket is retained before eviction;
	// cleanupInterval is how often the background sweep runs. Both guard the
	// clients map against unbounded growth from churning client IPs.
	ttl             time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	stopOnce        sync.Once
	maxClients      int
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		rate:            rps,
		burst:           float64(burst),
		clients:         make(map[string]*clientBucket),
		ttl:             10 * time.Minute,
		cleanupInterval: time.Minute,
		stopCleanup:     make(chan struct{}),
		maxClients:      defaultMaxClients,
	}
	go rl.cleanupLoop()
	return rl
}

// Stop terminates the background cleanup goroutine. Safe to call multiple times.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stopCleanup)
	})
}

// Middleware returns a Gin HandlerFunc applying rate limiting. Requests to
// exemptPaths (e.g. load-balancer health probes and metrics scrapes) are never
// limited, so monitoring keeps working while clients are being throttled.
func (rl *RateLimiter) Middleware(exemptPaths ...string) gin.HandlerFunc {
	exempt := make(map[string]bool, len(exemptPaths))
	for _, p := range exemptPaths {
		exempt[p] = true
	}
	return func(c *gin.Context) {
		if exempt[c.Request.URL.Path] {
			c.Next()
			return
		}
		ok, wait := rl.take(c.ClientIP())
		if !ok {
			c.Header("Retry-After", strconv.Itoa(retryAfterSeconds(wait)))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, models.NewErrorResponse(
				"RATE_LIMIT_EXCEEDED",
				"Too many requests. Please try again later.",
			))
			return
		}
		c.Next()
	}
}

func retryAfterSeconds(wait time.Duration) int {
	secs := int(math.Ceil(wait.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return secs
}

func (rl *RateLimiter) allow(ip string) bool {
	ok, _ := rl.take(ip)
	return ok
}

// take consumes a token for ip. When the request is rejected it also returns
// how long until the next token becomes available.
func (rl *RateLimiter) take(ip string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.clients[ip]
	if !exists {
		if len(rl.clients) >= rl.maxClients && !rl.evictRefilledLocked(now) {
			// Every tracked client is actively being limited: fail closed.
			return false, time.Second
		}
		rl.clients[ip] = &clientBucket{
			tokens:    rl.burst - 1,
			lastCheck: now,
		}
		return true, 0
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastCheck).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > rl.burst {
		bucket.tokens = rl.burst
	}
	bucket.lastCheck = now

	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true, 0
	}

	wait := time.Duration((1.0 - bucket.tokens) / rl.rate * float64(time.Second))
	return false, wait
}

// evictRefilledLocked drops buckets that have refilled to full capacity.
// Such a bucket is indistinguishable from a brand-new one, so eviction is
// lossless; it lets new clients in when the table is full of idle entries
// instead of rejecting them until the periodic sweep runs. Reports whether
// anything was evicted. Callers must hold rl.mu.
func (rl *RateLimiter) evictRefilledLocked(now time.Time) bool {
	evicted := false
	for ip, b := range rl.clients {
		if b.tokens+now.Sub(b.lastCheck).Seconds()*rl.rate >= rl.burst {
			delete(rl.clients, ip)
			evicted = true
		}
	}
	return evicted
}

// cleanup evicts buckets that have been idle longer than ttl.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.ttl)
	for ip, bucket := range rl.clients {
		if bucket.lastCheck.Before(cutoff) {
			delete(rl.clients, ip)
		}
	}
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stopCleanup:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

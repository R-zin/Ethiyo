package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/gin-gonic/gin"
)

type clientBucket struct {
	tokens    float64
	lastCheck time.Time
}

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

// Middleware returns a Gin HandlerFunc applying rate limiting.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, models.NewErrorResponse(
				"RATE_LIMIT_EXCEEDED",
				"Too many requests. Please try again later.",
			))
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.clients[ip]
	if !exists {
		rl.clients[ip] = &clientBucket{
			tokens:    rl.burst - 1,
			lastCheck: now,
		}
		return true
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
		return true
	}

	return false
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

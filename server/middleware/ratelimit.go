package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// bucket represents a token-bucket rate limiter for a single key.
type bucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func (b *bucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimiter holds per-key buckets.
type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	maxTokens  float64
	refillRate float64
}

// NewRateLimiter creates a rate limiter.
// maxTokens = burst capacity, refillRate = tokens per second.
func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:    make(map[string]*bucket),
		maxTokens:  maxTokens,
		refillRate: refillRate,
	}
	// Cleanup stale buckets every 5 minutes
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for k, b := range rl.buckets {
			if b.lastRefill.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a request from the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     rl.maxTokens,
			maxTokens:  rl.maxTokens,
			refillRate: rl.refillRate,
			lastRefill: time.Now(),
		}
		rl.buckets[key] = b
	}
	return b.allow()
}

// clientIP extracts the real client IP, respecting X-Forwarded-For.
func clientIP(c *gin.Context) string {
	return c.ClientIP()
}

// --- Pre-built rate limiters for different endpoint tiers ---

var (
	// GeneralLimiter: 60 requests per minute (1/s burst 60)
	GeneralLimiter = NewRateLimiter(60, 1.0)

	// ChatLimiter: 20 messages per minute (stricter, each triggers LLM call)
	ChatLimiter = NewRateLimiter(20, 0.33)

	// SubmitLimiter: IP-level general protection for submit endpoint
	SubmitLimiter = NewRateLimiter(10, 0.2)

	// ClawSubmitLimiter: 1 fragment per 5 minutes per Claw (quality over quantity)
	// maxTokens=1 (no burst), refillRate=1/300 (one token every 300 seconds)
	ClawSubmitLimiter = NewRateLimiter(1, 1.0/300.0)

	// RegisterLimiter: 5 registrations per minute (very strict)
	RegisterLimiter = NewRateLimiter(5, 0.08)

	// SessionLimiter: 10 session creations per minute
	SessionLimiter = NewRateLimiter(10, 0.17)
)

// RateLimit returns a Gin middleware that applies the given limiter by client IP.
func RateLimit(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := clientIP(c)
		if !limiter.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please try again later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitByKey returns a middleware that uses a custom key extractor.
// Useful for limiting by API key, wallet address, etc.
func RateLimitByKey(limiter *RateLimiter, keyFn func(c *gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			key = clientIP(c) // fallback to IP
		}
		if !limiter.Allow(key) {
			// Calculate seconds until next token
			waitSecs := int(1.0 / limiter.refillRate)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"message":     fmt.Sprintf("Quality over quantity — you can submit 1 fragment every %d minutes. Please take time to research and analyze deeply before your next submission.", waitSecs/60),
				"retry_after": waitSecs,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

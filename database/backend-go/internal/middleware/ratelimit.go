package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Simple in-memory rate limiter
type RateLimiter struct {
	requests map[string]*clientLimit
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

type clientLimit struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*clientLimit),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}

	// Cleanup goroutine to prevent memory leaks
	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, limit := range rl.requests {
			if now.After(limit.resetTime.Add(5 * time.Minute)) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware returns a Gin middleware function for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl == nil || rl.limit <= 0 || rl.window <= 0 {
			c.Next()
			return
		}
		ip := c.ClientIP()

		rl.mu.Lock()
		defer rl.mu.Unlock()

		now := time.Now()
		limit, exists := rl.requests[ip]

		if !exists || now.After(limit.resetTime) {
			rl.requests[ip] = &clientLimit{
				count:     1,
				resetTime: now.Add(rl.window),
			}
			c.Next()
			return
		}

		if limit.count >= rl.limit {
			c.Header("X-RateLimit-Limit", strconv.Itoa(rl.limit))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", limit.resetTime.Format(time.RFC3339))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			c.Abort()
			return
		}

		limit.count++
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(rl.limit-limit.count))
		c.Next()
	}
}

// NewAuthRateLimiter creates a stricter rate limiter for authentication endpoints.
// Disabled by default to allow unlimited attempts.
func NewAuthRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*clientLimit),
		limit:    0,
		window:   15 * time.Minute,
	}

	// Cleanup goroutine to prevent memory leaks
	go rl.cleanup()

	return rl
}

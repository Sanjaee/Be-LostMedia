package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter stores rate limiters per IP
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rps      rate.Limit
	burst    int
	cleanup  *time.Ticker
}

// NewRateLimiter creates a new rate limiter middleware
func NewRateLimiter(rps int, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		cleanup:  time.NewTicker(5 * time.Minute), // Cleanup every 5 minutes
	}

	// Start cleanup goroutine to remove old limiters
	go rl.cleanupLimiters()

	return rl
}

// getLimiter returns the rate limiter for a given IP
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rps, rl.burst)
		rl.limiters[ip] = limiter
	}

	return limiter
}

// cleanupLimiters periodically removes old limiters to prevent memory leak
func (rl *RateLimiter) cleanupLimiters() {
	for range rl.cleanup.C {
		// In a production system, you might want to track last access time
		// For simplicity, we'll just limit the map size
		rl.mu.Lock()
		if len(rl.limiters) > 10000 {
			// Clear half of the limiters (simple cleanup strategy)
			newLimiters := make(map[string]*rate.Limiter)
			count := 0
			for k, v := range rl.limiters {
				if count < 5000 {
					newLimiters[k] = v
					count++
				}
			}
			rl.limiters = newLimiters
		}
		rl.mu.Unlock()
	}
}

// Middleware returns the rate limiter middleware function
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		ip := c.ClientIP()
		if ip == "" {
			ip = c.RemoteIP()
		}

		// Get limiter for this IP
		limiter := rl.getLimiter(ip)

		// Check if request is allowed
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Too many requests",
				"message": "Rate limit exceeded. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Stop stops the cleanup ticker
func (rl *RateLimiter) Stop() {
	rl.cleanup.Stop()
}

package auth

import (
"sync"
"time"
)

// RateLimiter implements per-key rate limiting using token bucket algorithm.
type RateLimiter struct {
mu      sync.Mutex
buckets map[string]*tokenBucket
rate    int
window  time.Duration
}

type tokenBucket struct {
tokens    int
lastFill  time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(ratePerWindow int, window time.Duration) *RateLimiter {
return &RateLimiter{
buckets: make(map[string]*tokenBucket),
rate:    ratePerWindow,
window:  window,
}
}

// Allow checks if a request should be allowed for the given key.
// Returns (allowed, retryAfter) where retryAfter is the duration to wait if denied.
func (rl *RateLimiter) Allow(key string) (bool, time.Duration) {
rl.mu.Lock()
defer rl.mu.Unlock()

now := time.Now()
bucket, exists := rl.buckets[key]

if !exists {
rl.buckets[key] = &tokenBucket{
tokens:   rl.rate - 1, // Consume one token
lastFill: now,
}
return true, 0
}

// Refill tokens based on elapsed time
elapsed := now.Sub(bucket.lastFill)
refill := int(float64(elapsed) / float64(rl.window) * float64(rl.rate))

if refill > 0 {
bucket.tokens = minInt(rl.rate, bucket.tokens+refill)
bucket.lastFill = now
}

if bucket.tokens > 0 {
bucket.tokens--
return true, 0
}

// Calculate retry-after
tokenTime := rl.window / time.Duration(rl.rate)
return false, tokenTime
}

// Reset resets the rate limiter for a key (useful for testing).
func (rl *RateLimiter) Reset(key string) {
rl.mu.Lock()
defer rl.mu.Unlock()
delete(rl.buckets, key)
}

func minInt(a, b int) int {
if a < b {
return a
}
return b
}

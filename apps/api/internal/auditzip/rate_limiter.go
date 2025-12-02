package auditzip

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu        sync.Mutex
	perTenant map[string]*tenantRate
	limit     int
	window    time.Duration
}

type tenantRate struct {
	count       int
	windowStart time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		return &RateLimiter{limit: 0}
	}
	return &RateLimiter{
		perTenant: map[string]*tenantRate{},
		limit:     limit,
		window:    window,
	}
}

func (r *RateLimiter) Allow(tenant string) (bool, time.Duration) {
	if r == nil || r.limit == 0 {
		return true, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	state, ok := r.perTenant[tenant]
	if !ok {
		state = &tenantRate{windowStart: now}
		r.perTenant[tenant] = state
	}
	if now.Sub(state.windowStart) >= r.window {
		state.windowStart = now
		state.count = 0
	}
	if state.count >= r.limit {
		return false, state.windowStart.Add(r.window).Sub(now)
	}
	state.count++
	return true, 0
}

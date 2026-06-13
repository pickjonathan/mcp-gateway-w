// Package quota enforces per-organization and per-user request limits
// (noisy-neighbor protection, FR-017). It uses an in-memory fixed-window
// counter for dev/tests; a Redis-backed limiter can slot in behind Enforcer for
// a multi-instance gateway fleet.
package quota

import (
	"sync"
	"time"
)

type bucket struct {
	start time.Time
	count int
}

// WindowLimiter is a fixed-window per-key counter. A limit <= 0 means unlimited.
type WindowLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*bucket
	now     func() time.Time
}

// NewWindowLimiter returns a limiter allowing `limit` requests per `window` per key.
func NewWindowLimiter(limit int, window time.Duration) *WindowLimiter {
	return &WindowLimiter{limit: limit, window: window, buckets: make(map[string]*bucket), now: time.Now}
}

// Allow records a request for key and reports whether it is within the limit.
func (l *WindowLimiter) Allow(key string) bool {
	if l.limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	ws := l.now().Truncate(l.window)
	b := l.buckets[key]
	if b == nil || !b.start.Equal(ws) {
		b = &bucket{start: ws}
		l.buckets[key] = b
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}

// limiter is a per-key request counter. Implementations: in-memory WindowLimiter
// (dev/single instance) and RedisLimiter (fleet-wide).
type limiter interface {
	Allow(key string) bool
}

// Enforcer applies org- and user-scoped limits. A nil *Enforcer is unlimited.
type Enforcer struct {
	org  limiter
	user limiter
}

// NewEnforcer builds an enforcer; a 0 limit means unlimited for that scope.
func NewEnforcer(orgPerWindow, userPerWindow int, window time.Duration) *Enforcer {
	return &Enforcer{
		org:  NewWindowLimiter(orgPerWindow, window),
		user: NewWindowLimiter(userPerWindow, window),
	}
}

// Allow reports whether a request from (org, user) is within both the org and
// user limits. Limits are keyed independently, so one tenant exhausting its
// quota never affects another (noisy-neighbor isolation).
func (e *Enforcer) Allow(org, user string) bool {
	if e == nil {
		return true
	}
	if !e.org.Allow("org:" + org) {
		return false
	}
	return e.user.Allow("user:" + org + "/" + user)
}

package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// incrExpire atomically increments a window counter and sets its TTL on creation,
// so a crash between INCR and EXPIRE can never leave an immortal key.
var incrExpire = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return c
`)

// RedisLimiter is a fixed-window per-key counter shared across gateway replicas
// via Redis, so a tenant's limit is fleet-wide rather than per-instance. A limit
// <= 0 means unlimited. It fails open: if Redis is unreachable the request is
// allowed (availability over strict enforcement — a soft goal must never take
// down the data plane, which still upholds the hard constraints).
type RedisLimiter struct {
	rdb    redis.Scripter
	limit  int
	window time.Duration
	now    func() time.Time
}

// NewRedisLimiter returns a Redis-backed fixed-window limiter.
func NewRedisLimiter(rdb redis.Scripter, limit int, window time.Duration) *RedisLimiter {
	return &RedisLimiter{rdb: rdb, limit: limit, window: window, now: time.Now}
}

// Allow records a request for key in the current window and reports whether it is
// within the limit. The window key embeds the window index so it auto-expires.
func (l *RedisLimiter) Allow(key string) bool {
	if l.limit <= 0 {
		return true
	}
	windowMs := l.window.Milliseconds()
	if windowMs <= 0 {
		return true
	}
	idx := l.now().UnixMilli() / windowMs
	rkey := fmt.Sprintf("quota:%s:%d", key, idx)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	n, err := incrExpire.Run(ctx, l.rdb, []string{rkey}, windowMs).Int()
	if err != nil {
		return true // fail open
	}
	return n <= l.limit
}

// NewRedisEnforcer builds an Enforcer whose org and user windows are shared across
// instances via Redis. A 0 limit means unlimited for that scope.
func NewRedisEnforcer(rdb redis.Scripter, orgPerWindow, userPerWindow int, window time.Duration) *Enforcer {
	return &Enforcer{
		org:  NewRedisLimiter(rdb, orgPerWindow, window),
		user: NewRedisLimiter(rdb, userPerWindow, window),
	}
}

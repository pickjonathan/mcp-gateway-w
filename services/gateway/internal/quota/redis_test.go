package quota

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// A 0 limit is unlimited and never touches Redis (safe with a nil client).
func TestRedisLimiter_Unlimited(t *testing.T) {
	l := NewRedisLimiter(nil, 0, time.Minute)
	for i := 0; i < 5; i++ {
		if !l.Allow("k") {
			t.Fatal("limit <= 0 must always allow")
		}
	}
}

// When Redis is unreachable the limiter fails open (availability over strict
// enforcement) rather than blocking the data plane.
func TestRedisLimiter_FailOpen(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}) // nothing listening
	defer rdb.Close()
	l := NewRedisLimiter(rdb, 1, time.Minute)
	// Two calls against a limit of 1: with Redis down it must allow both
	// (fail open). Separate calls so the two side-effecting checks are distinct.
	first := l.Allow("k")
	second := l.Allow("k")
	if !first || !second {
		t.Fatal("limiter must fail open when Redis is unreachable")
	}
}

// TestRedisLimiter_SharedWindow proves the limit is fleet-wide: two limiters
// sharing one Redis (two gateway replicas) enforce a single combined window.
func TestRedisLimiter_SharedWindow(t *testing.T) {
	addr := os.Getenv("MCP_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set MCP_TEST_REDIS_ADDR to run the Redis limiter test")
	}
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	key := fmt.Sprintf("test/%d", time.Now().UnixNano()) // unique per run
	a := NewRedisLimiter(rdb, 3, time.Minute)
	b := NewRedisLimiter(rdb, 3, time.Minute)
	// Pin both to the same window so the test can't straddle a window boundary.
	fixed := time.Now()
	a.now = func() time.Time { return fixed }
	b.now = func() time.Time { return fixed }

	got := []bool{a.Allow(key), b.Allow(key), a.Allow(key), b.Allow(key), a.Allow(key)}
	want := []bool{true, true, true, false, false} // 3 allowed across both instances
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("request %d: got %v want %v (all=%v)", i, got[i], want[i], got)
		}
	}

	// A different key has its own window (noisy-neighbor isolation).
	other := NewRedisLimiter(rdb, 3, time.Minute)
	other.now = func() time.Time { return fixed }
	otherKey := key + "-other"
	if !other.Allow(otherKey) {
		t.Fatal("a different key must have an independent window")
	}

	idx := fixed.UnixMilli() / time.Minute.Milliseconds()
	rdb.Del(ctx,
		fmt.Sprintf("quota:%s:%d", key, idx),
		fmt.Sprintf("quota:%s:%d", otherKey, idx),
	)
}

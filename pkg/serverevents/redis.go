package serverevents

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// RedisBus is a Redis pub/sub implementation of Bus (control-plane → gateway).
type RedisBus struct{ rdb *redis.Client }

// NewRedisBus connects to Redis at addr.
func NewRedisBus(addr string) *RedisBus {
	return &RedisBus{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

// Publish marshals e and publishes it on Channel.
func (b *RedisBus) Publish(ctx context.Context, e Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, Channel, data).Err()
}

// Subscribe consumes events from Channel until ctx is cancelled. Note: pub/sub
// is fire-and-forget; a DB-backed reconcile on startup is the durability
// backstop (added with the persistence layer, T007).
func (b *RedisBus) Subscribe(ctx context.Context, handler func(Event)) error {
	sub := b.rdb.Subscribe(ctx, Channel)
	defer func() { _ = sub.Close() }()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var e Event
			if err := json.Unmarshal([]byte(msg.Payload), &e); err == nil {
				handler(e)
			}
		}
	}
}

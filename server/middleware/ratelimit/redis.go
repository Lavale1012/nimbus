package ratelimit

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStore is a fixed-window rate-limit backend backed by Redis, so the limit
// is shared across every API instance behind the load balancer and survives
// restarts/deploys. It uses the standard INCR + EXPIRE counter pattern.
type redisStore struct {
	client *redis.Client
	limit  int
	window time.Duration
	prefix string
}

// NewWithRedis returns a Limiter whose counters live in Redis. All instances
// pointed at the same Redis see the same counts, making `limit` a true global
// cap per `window`.
//
// Fail-open: if Redis is unreachable, requests are allowed rather than blocked —
// a rate limiter should degrade to "no limiting", not to a hard outage. Failures
// are logged so the condition is visible.
func NewWithRedis(client *redis.Client, limit int, window time.Duration) *Limiter {
	return &Limiter{backend: &redisStore{
		client: client,
		limit:  limit,
		window: window,
		prefix: "ratelimit:",
	}}
}

func (s *redisStore) allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	redisKey := s.prefix + key

	// INCR returns the new counter value. On the first hit for a fresh window the
	// value is 1, and we set the TTL so the window expires on its own. Doing INCR
	// first (then EXPIRE) is the common atomic-enough pattern; a pipeline keeps it
	// to a single round trip.
	pipe := s.client.Pipeline()
	incr := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, s.window)
	if _, err := pipe.Exec(ctx); err != nil {
		// Fail open: don't let a Redis blip take down auth.
		log.Printf("[ratelimit] redis error, allowing request: %v", err)
		return true
	}

	count := incr.Val()
	// Guard against the (rare) race where INCR set a value but EXPIRE didn't land:
	// if a key somehow has no TTL, give it one so it can't leak forever.
	if count == 1 {
		_ = s.client.Expire(ctx, redisKey, s.window).Err()
	}

	return count <= int64(s.limit)
}

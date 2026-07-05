// Package redis provides an optional Redis connection for the API server. Redis
// backs the cross-instance rate limiter; when REDIS_ADDR is unset the server
// falls back to in-memory limiting, so Redis is not required for local dev.
package redis

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Connect returns a Redis client if REDIS_ADDR is set (host:port), pinging it to
// verify connectivity. It returns (nil, nil) when REDIS_ADDR is unset — a signal
// to the caller to use the in-memory limiter instead. REDIS_PASSWORD is applied
// when present.
func Connect(ctx context.Context) (*redis.Client, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return nil, nil // no Redis configured; caller falls back to in-memory
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", addr, err)
	}
	return client, nil
}

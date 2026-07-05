package ratelimit

import (
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// newMiniRedis spins up an in-process Redis fake and a client pointed at it.
func newMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestRedisLimiter_BlocksAfterLimit(t *testing.T) {
	_, client := newMiniRedis(t)
	l := NewWithRedis(client, 3, time.Minute)
	r := testRouter(l, nil)

	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, post(r, "1.2.3.4", nil).Code, "attempt %d", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, post(r, "1.2.3.4", nil).Code)
}

func TestRedisLimiter_SeparateKeysIndependent(t *testing.T) {
	_, client := newMiniRedis(t)
	l := NewWithRedis(client, 1, time.Minute)
	r := testRouter(l, nil)

	assert.Equal(t, http.StatusOK, post(r, "1.1.1.1", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, post(r, "1.1.1.1", nil).Code)
	assert.Equal(t, http.StatusOK, post(r, "2.2.2.2", nil).Code)
}

func TestRedisLimiter_WindowExpires(t *testing.T) {
	mr, client := newMiniRedis(t)
	l := NewWithRedis(client, 1, time.Minute)
	r := testRouter(l, nil)

	assert.Equal(t, http.StatusOK, post(r, "9.9.9.9", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, post(r, "9.9.9.9", nil).Code)

	// Advance miniredis past the window so the counter TTL expires.
	mr.FastForward(61 * time.Second)
	assert.Equal(t, http.StatusOK, post(r, "9.9.9.9", nil).Code)
}

// A shared Redis backend means two independent Limiter instances (as different
// API tasks would have) enforce ONE combined limit — the core reason for #9.
func TestRedisLimiter_SharedAcrossInstances(t *testing.T) {
	_, client := newMiniRedis(t)
	limit := 2
	inst1 := NewWithRedis(client, limit, time.Minute)
	inst2 := NewWithRedis(client, limit, time.Minute)
	r1 := testRouter(inst1, nil)
	r2 := testRouter(inst2, nil)

	// Same key hitting two different "instances": the 3rd request is blocked
	// regardless of which instance serves it.
	assert.Equal(t, http.StatusOK, post(r1, "7.7.7.7", nil).Code)
	assert.Equal(t, http.StatusOK, post(r2, "7.7.7.7", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, post(r1, "7.7.7.7", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, post(r2, "7.7.7.7", nil).Code)
}

// If Redis is unreachable the limiter must fail open (allow), not hard-block.
func TestRedisLimiter_FailsOpenOnRedisError(t *testing.T) {
	mr, client := newMiniRedis(t)
	l := NewWithRedis(client, 1, time.Minute)
	r := testRouter(l, nil)

	mr.Close() // kill Redis

	// With Redis down, requests are allowed rather than 429'd.
	assert.Equal(t, http.StatusOK, post(r, "5.5.5.5", nil).Code)
	assert.Equal(t, http.StatusOK, post(r, "5.5.5.5", nil).Code)
}

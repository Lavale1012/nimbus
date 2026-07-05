// Package ratelimit provides a per-key rate limiter for Gin routes. It is used
// to slow brute-force attempts against the authentication endpoints (login and
// password reset), where an attacker might otherwise guess the short
// 4-character passkey.
//
// The limiter is a fixed-window counter keyed by a caller-supplied string
// (typically client IP + email). Each key is allowed `limit` attempts per
// `window`; once exceeded, requests receive HTTP 429 until the window rolls over.
//
// Two backends are available behind the same store interface:
//   - New()          — in-memory, per-process (resets on restart; fine for a
//     single instance or local development).
//   - NewWithRedis() — shared across instances via Redis, so the limit is a
//     true global cap behind a load balancer and survives deploys.
package ratelimit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// store is the pluggable backend for the fixed-window counter. allow records one
// attempt against key and reports whether the request is within the configured
// limit for the current window.
type store interface {
	allow(key string) bool
}

// Limiter rate-limits requests using a store backend. It is safe for concurrent
// use (both backends are).
type Limiter struct {
	backend store
}

// New returns a Limiter backed by an in-memory store, allowing `limit` requests
// per `window` for each key. State is per-process and resets on restart.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{backend: newMemoryStore(limit, window)}
}

// ── In-memory backend ────────────────────────────────────────────────────────

// counter tracks the number of attempts for one key within the current window.
type counter struct {
	count       int
	windowStart time.Time
}

type memoryStore struct {
	mu       sync.Mutex
	counters map[string]*counter
	limit    int
	window   time.Duration
}

func newMemoryStore(limit int, window time.Duration) *memoryStore {
	s := &memoryStore{
		counters: make(map[string]*counter),
		limit:    limit,
		window:   window,
	}
	go s.cleanupLoop()
	return s
}

func (s *memoryStore) allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	c, ok := s.counters[key]
	if !ok || now.Sub(c.windowStart) >= s.window {
		s.counters[key] = &counter{count: 1, windowStart: now}
		return true
	}

	if c.count >= s.limit {
		return false
	}
	c.count++
	return true
}

// cleanupLoop removes counters whose window has fully elapsed. It runs for the
// lifetime of the process; there is no need to stop it in this application.
func (s *memoryStore) cleanupLoop() {
	ticker := time.NewTicker(s.window)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for key, c := range s.counters {
			if now.Sub(c.windowStart) >= s.window {
				delete(s.counters, key)
			}
		}
		s.mu.Unlock()
	}
}

// Middleware returns a Gin handler that rate-limits a request against one or
// more independent buckets. keysFunc derives the keys for a request; the request
// is rejected if ANY of those keys is over its limit. This lets a single
// middleware enforce, e.g., a per-IP budget and a per-email budget at once, so
// an attacker rotating IPs against one account is still caught by the email
// bucket (and vice versa).
//
// If keysFunc is nil, the client IP is used as the sole key.
func (l *Limiter) Middleware(keysFunc func(c *gin.Context) []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var keys []string
		if keysFunc != nil {
			keys = keysFunc(c)
		}
		if len(keys) == 0 {
			keys = []string{c.ClientIP()}
		}

		// Record an attempt against every bucket, and block if any is exceeded.
		// We evaluate all keys (not short-circuiting) so each bucket counts this
		// attempt, which is the conservative choice for abuse tracking.
		blocked := false
		for _, k := range keys {
			if !l.backend.allow(k) {
				blocked = true
			}
		}
		if blocked {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many attempts. Please try again later.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// IPAndEmailKeys derives independent rate-limit buckets from the client IP and,
// when present, the email in the JSON body. The body is peeked without being
// consumed — it is restored so the downstream handler can still bind it.
//
// Returning both keys means the limiter throttles per source IP AND per targeted
// account. When the body has no email, only the IP bucket is used.
func IPAndEmailKeys(c *gin.Context) []string {
	keys := []string{"ip:" + c.ClientIP()}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return keys
	}
	// Restore the body for the actual handler.
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Email != "" {
		keys = append(keys, "email:"+payload.Email)
	}
	return keys
}

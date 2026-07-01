// Package ratelimit provides a small, dependency-free per-key rate limiter for
// Gin routes. It is used to slow brute-force attempts against the authentication
// endpoints (login and password reset), where an attacker might otherwise guess
// the short 4-character passkey.
//
// The limiter is a fixed-window counter keyed by a caller-supplied string
// (typically client IP + email). Each key is allowed `limit` attempts per
// `window`; once exceeded, requests receive HTTP 429 until the window rolls over.
// State is kept in memory, so it resets on restart and is per-process — adequate
// for a single API instance. A background sweeper evicts stale keys so memory
// does not grow unbounded.
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

// counter tracks the number of attempts for one key within the current window.
type counter struct {
	count       int
	windowStart time.Time
}

// Limiter is a fixed-window, per-key rate limiter safe for concurrent use.
type Limiter struct {
	mu       sync.Mutex
	counters map[string]*counter
	limit    int
	window   time.Duration
}

// New returns a Limiter allowing `limit` requests per `window` for each key.
// It starts a background goroutine that periodically evicts expired entries.
func New(limit int, window time.Duration) *Limiter {
	l := &Limiter{
		counters: make(map[string]*counter),
		limit:    limit,
		window:   window,
	}
	go l.cleanupLoop()
	return l
}

// allow records an attempt for key and reports whether it is within the limit.
func (l *Limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	c, ok := l.counters[key]
	if !ok || now.Sub(c.windowStart) >= l.window {
		l.counters[key] = &counter{count: 1, windowStart: now}
		return true
	}

	if c.count >= l.limit {
		return false
	}
	c.count++
	return true
}

// cleanupLoop removes counters whose window has fully elapsed. It runs for the
// lifetime of the process; there is no need to stop it in this application.
func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(l.window)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for key, c := range l.counters {
			if now.Sub(c.windowStart) >= l.window {
				delete(l.counters, key)
			}
		}
		l.mu.Unlock()
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
			if !l.allow(k) {
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

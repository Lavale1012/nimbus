// Package bodylimit provides Gin middleware that caps the size of incoming
// request bodies. Without a limit, a client can POST an arbitrarily large body
// to any endpoint and force the server to read it into memory. This middleware
// wraps the request body in http.MaxBytesReader so any read past the limit fails
// and the connection is cut, bounding per-request memory use.
//
// It targets JSON/API request bodies (login, register, password reset, etc.).
// File uploads are not affected because they go directly to S3 via presigned
// URLs and never pass their bytes through this server.
package bodylimit

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DefaultMaxBytes is a sensible cap for the JSON bodies this API accepts. The
// largest legitimate request is a handful of short string fields, so 1 MiB is
// already very generous.
const DefaultMaxBytes int64 = 1 << 20 // 1 MiB

// Middleware returns a Gin handler that limits each request body to maxBytes.
// A body larger than the limit causes subsequent reads (e.g. c.ShouldBindJSON)
// to fail; handlers already return 400 on bind errors, so oversized requests are
// rejected without the full body ever being buffered.
//
// If maxBytes <= 0, DefaultMaxBytes is used.
func Middleware(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

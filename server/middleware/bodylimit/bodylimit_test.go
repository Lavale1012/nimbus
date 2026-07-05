package bodylimit

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// router mounts the middleware and a handler that fully reads the body, so an
// oversized body triggers MaxBytesReader during the read.
func router(maxBytes int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", Middleware(maxBytes), func(c *gin.Context) {
		var payload struct {
			Data string `json:"data"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad body"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func post(r *gin.Engine, body []byte) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(http.MethodPost, "/x", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestBodyLimit_AllowsSmallBody(t *testing.T) {
	r := router(1 << 20) // 1 MiB
	w := post(r, []byte(`{"data":"hello"}`))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBodyLimit_RejectsOversizedBody(t *testing.T) {
	limit := int64(64) // tiny limit for the test
	r := router(limit)

	// Build a JSON body comfortably larger than the limit.
	big := `{"data":"` + strings.Repeat("A", 500) + `"}`
	w := post(r, []byte(big))

	// The read fails past the limit, so the handler's bind fails -> 400.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBodyLimit_DefaultWhenNonPositive(t *testing.T) {
	// maxBytes <= 0 should fall back to DefaultMaxBytes, not block everything.
	r := router(0)
	w := post(r, []byte(`{"data":"hello"}`))
	assert.Equal(t, http.StatusOK, w.Code)
}

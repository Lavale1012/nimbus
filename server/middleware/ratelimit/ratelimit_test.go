package ratelimit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func testRouter(l *Limiter, keysFunc func(c *gin.Context) []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", l.Middleware(keysFunc), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func post(r *gin.Engine, ip string, body map[string]string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/x", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLimiter_BlocksAfterLimit(t *testing.T) {
	l := New(3, time.Minute)
	r := testRouter(l, nil)

	for i := 0; i < 3; i++ {
		w := post(r, "1.2.3.4", nil)
		assert.Equal(t, http.StatusOK, w.Code, "attempt %d should pass", i+1)
	}
	// 4th attempt within the window is blocked.
	w := post(r, "1.2.3.4", nil)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestLimiter_SeparateKeysIndependent(t *testing.T) {
	l := New(1, time.Minute)
	r := testRouter(l, nil)

	assert.Equal(t, http.StatusOK, post(r, "1.1.1.1", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, post(r, "1.1.1.1", nil).Code)
	// A different IP has its own budget.
	assert.Equal(t, http.StatusOK, post(r, "2.2.2.2", nil).Code)
}

func TestLimiter_WindowResets(t *testing.T) {
	l := New(1, 50*time.Millisecond)
	r := testRouter(l, nil)

	assert.Equal(t, http.StatusOK, post(r, "9.9.9.9", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, post(r, "9.9.9.9", nil).Code)

	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, http.StatusOK, post(r, "9.9.9.9", nil).Code)
}

func TestIPAndEmailKeys_ThrottlesPerAccountAcrossIPs(t *testing.T) {
	l := New(1, time.Minute)
	r := testRouter(l, IPAndEmailKeys)

	body := map[string]string{"email": "victim@example.com"}
	// Same email from two different IPs — second is still blocked because the
	// per-email bucket is exhausted even though the IP differs.
	assert.Equal(t, http.StatusOK, post(r, "10.0.0.1", body).Code)
	assert.Equal(t, http.StatusTooManyRequests, post(r, "10.0.0.2", body).Code)
}

func TestIPAndEmailKeys_ThrottlesPerIPAcrossAccounts(t *testing.T) {
	l := New(1, time.Minute)
	r := testRouter(l, IPAndEmailKeys)

	// Same IP targeting two different emails — second is blocked by the per-IP
	// bucket.
	assert.Equal(t, http.StatusOK, post(r, "11.0.0.1", map[string]string{"email": "a@example.com"}).Code)
	assert.Equal(t, http.StatusTooManyRequests, post(r, "11.0.0.1", map[string]string{"email": "b@example.com"}).Code)
}

func TestIPAndEmailKeys_PreservesBodyForHandler(t *testing.T) {
	l := New(5, time.Minute)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", l.Middleware(IPAndEmailKeys), func(c *gin.Context) {
		var payload struct {
			Email string `json:"email"`
		}
		// The handler must still be able to read the body the middleware peeked.
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad body"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"email": payload.Email})
	})

	w := post(r, "10.0.0.5", map[string]string{"email": "keep@example.com"})
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "keep@example.com", resp["email"])
}

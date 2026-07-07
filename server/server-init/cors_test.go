package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestResolveCORS(t *testing.T) {
	cases := []struct {
		name        string
		localDev    bool
		corsOrigins string
		wantOrigins []string
		wantCreds   bool
		wantEnabled bool
	}{
		{"local dev allows all", true, "", []string{"*"}, false, true},
		{"local dev ignores CORS_ORIGINS", true, "https://x.com", []string{"*"}, false, true},
		{"prod with allowlist", false, "https://app.example.com", []string{"https://app.example.com"}, true, true},
		{"prod with multiple origins", false, "https://a.com,https://b.com", []string{"https://a.com", "https://b.com"}, true, true},
		{"prod unset -> disabled", false, "", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origins, creds, enabled := resolveCORS(tc.localDev, tc.corsOrigins)
			assert.Equal(t, tc.wantEnabled, enabled)
			assert.Equal(t, tc.wantCreds, creds)
			assert.Equal(t, tc.wantOrigins, origins)
		})
	}
}

// router mirrors how InitServer wires CORS: register the middleware only when
// resolveCORS says it's enabled.
func corsRouter(localDev bool, corsOrigins string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	origins, creds, enabled := resolveCORS(localDev, corsOrigins)
	if enabled {
		r.Use(cors.New(corsConfig(origins, creds)))
	}
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func corsHeaderFor(t *testing.T, r *gin.Engine, origin string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Header().Get("Access-Control-Allow-Origin")
}

func TestCORS_LocalDevAllowsAnyOrigin(t *testing.T) {
	r := corsRouter(true, "")
	assert.Equal(t, "*", corsHeaderFor(t, r, "https://evil.example.com"))
}

func TestCORS_ProdAllowlistEchoesAllowedOrigin(t *testing.T) {
	r := corsRouter(false, "https://app.example.com")
	assert.Equal(t, "https://app.example.com", corsHeaderFor(t, r, "https://app.example.com"))
}

func TestCORS_ProdAllowlistBlocksUnlistedOrigin(t *testing.T) {
	r := corsRouter(false, "https://app.example.com")
	// A non-allowlisted origin must NOT get an allow-origin header.
	assert.Empty(t, corsHeaderFor(t, r, "https://evil.example.com"))
}

func TestCORS_ProdUnsetSendsNoCORSHeader(t *testing.T) {
	// The core #8 guarantee: prod with no CORS_ORIGINS never emits a wildcard
	// (or any) Access-Control-Allow-Origin header.
	r := corsRouter(false, "")
	assert.Empty(t, corsHeaderFor(t, r, "https://anything.example.com"))
}

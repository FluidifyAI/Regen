package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCORS_AllowedOriginReceivesHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://incidents.myco.com")
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://incidents.myco.com")
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://incidents.myco.com" {
		t.Errorf("ACAO: want %q, got %q", "https://incidents.myco.com", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("ACAC: want true, got %q", got)
	}
}

func TestCORS_UnknownOriginReceivesNoHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://incidents.myco.com")
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://attacker.com")
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unknown origin should not receive ACAO header, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("unknown origin should not receive ACAC header, got %q", got)
	}
	// Request still served (not blocked at middleware level — browser enforces CORS)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestCORS_PreflightAllowedOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://incidents.myco.com")
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://incidents.myco.com")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight allowed: want 204, got %d", w.Code)
	}
}

func TestCORS_PreflightUnknownOriginForbidden(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://incidents.myco.com")
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://attacker.com")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("preflight unknown: want 403, got %d", w.Code)
	}
}

func TestCORS_MultipleAllowedOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://incidents.myco.com, http://localhost:3000")
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, origin := range []string{"https://incidents.myco.com", "http://localhost:3000"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", origin)
		r.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("origin %q: want ACAO=%q, got %q", origin, origin, got)
		}
	}
}

func TestCORS_DefaultAllowsLocalhost(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "") // unset → dev mode allows any localhost port
	t.Setenv("APP_ENV", "development")
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, origin := range []string{"http://localhost:3000", "http://localhost:3001", "http://localhost:5173"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", origin)
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("dev mode should allow %s, got %q", origin, got)
		}
	}
}

func TestCORS_ProductionBlocksUnlistedLocalhost(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	t.Setenv("APP_ENV", "production")
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3001") // not in allowlist
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("production should block unlisted origin, got %q", got)
	}
}

func TestParseAllowedOrigins(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"", nil}, // empty → no explicit allowlist; dev mode uses isLocalhost instead
		{"https://a.com", []string{"https://a.com"}},
		{"https://a.com, https://b.com", []string{"https://a.com", "https://b.com"}},
		{"  https://a.com  ,  https://b.com  ", []string{"https://a.com", "https://b.com"}},
	}
	for _, tc := range cases {
		got := parseAllowedOrigins(tc.input)
		if len(got) != len(tc.expected) {
			t.Errorf("input %q: want %v, got %v", tc.input, tc.expected, got)
			continue
		}
		for i := range got {
			if got[i] != tc.expected[i] {
				t.Errorf("input %q [%d]: want %q, got %q", tc.input, i, tc.expected[i], got[i])
			}
		}
	}
}

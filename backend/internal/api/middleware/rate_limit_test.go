package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	redispkg "github.com/openincident/openincident/internal/redis"
	"github.com/redis/go-redis/v9"

	"github.com/openincident/openincident/internal/api/middleware"
)

func setupMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	redispkg.Client = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { redispkg.Client = nil })
	return mr
}

func newTestRouter(handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(handler)
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	setupMiniredis(t)
	r := newTestRouter(middleware.RateLimit("test", 3, 60))

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimit_Blocks429WhenExceeded(t *testing.T) {
	setupMiniredis(t)
	r := newTestRouter(middleware.RateLimit("test", 2, 60))

	send := func() int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		return w.Code
	}

	if got := send(); got != http.StatusOK {
		t.Fatalf("req 1: want 200, got %d", got)
	}
	if got := send(); got != http.StatusOK {
		t.Fatalf("req 2: want 200, got %d", got)
	}
	if got := send(); got != http.StatusTooManyRequests {
		t.Fatalf("req 3: want 429, got %d", got)
	}
}

func TestRateLimit_SetsHeaders(t *testing.T) {
	setupMiniredis(t)
	r := newTestRouter(middleware.RateLimit("test", 5, 60))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w, req)

	if w.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("X-RateLimit-Limit: want 5, got %q", w.Header().Get("X-RateLimit-Limit"))
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("X-RateLimit-Remaining header missing")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("X-RateLimit-Reset header missing")
	}
}

func TestRateLimit_FailsOpenWhenRedisNil(t *testing.T) {
	// No miniredis — Client remains nil
	redispkg.Client = nil
	r := newTestRouter(middleware.RateLimit("test", 1, 60))

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("fail-open: want 200, got %d", w.Code)
		}
	}
}

func TestRateLimit_IsolatesPerIP(t *testing.T) {
	setupMiniredis(t)
	r := newTestRouter(middleware.RateLimit("test", 1, 60))

	send := func(ip string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip + ":1234"
		r.ServeHTTP(w, req)
		return w.Code
	}

	if got := send("1.1.1.1"); got != http.StatusOK {
		t.Fatalf("IP A req 1: want 200, got %d", got)
	}
	// IP A exhausted (limit=1), IP B should still work
	if got := send("1.1.1.1"); got != http.StatusTooManyRequests {
		t.Fatalf("IP A req 2: want 429, got %d", got)
	}
	if got := send("2.2.2.2"); got != http.StatusOK {
		t.Fatalf("IP B req 1: want 200, got %d", got)
	}
}

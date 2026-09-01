package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// withValidSpanContext injects a real (not mocked) trace.SpanContext into the
// request context before RequestID runs, simulating otelgin having already
// opened a span earlier in the middleware chain.
func withValidSpanContext(traceID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid, err := trace.TraceIDFromHex(traceID)
		if err != nil {
			panic(err) // test setup bug, not a runtime path
		}
		sc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tid,
			SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
			TraceFlags: trace.FlagsSampled,
		})
		ctx := trace.ContextWithSpanContext(c.Request.Context(), sc)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func newRequestIDEngine(preMiddleware ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	for _, mw := range preMiddleware {
		r.Use(mw)
	}
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) {
		id, _ := c.Get("request_id")
		c.String(http.StatusOK, "%v", id)
	})
	return r
}

func TestRequestID_UsesTraceIDWhenValidTraceExists(t *testing.T) {
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	r := newRequestIDEngine(withValidSpanContext(traceID))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("X-Request-ID"); got != traceID {
		t.Errorf("X-Request-ID header = %q, want the trace ID %q", got, traceID)
	}
	if got := rec.Body.String(); got != traceID {
		t.Errorf("request_id context value = %q, want the trace ID %q", got, traceID)
	}
}

func TestRequestID_TraceIDOverridesIncomingHeader(t *testing.T) {
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	r := newRequestIDEngine(withValidSpanContext(traceID))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-supplied-id-should-be-overridden")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != traceID {
		t.Errorf("X-Request-ID header = %q, want the trace ID %q to win once a real trace exists", got, traceID)
	}
}

func TestRequestID_FallsBackToUUIDWhenNoTraceAndNoHeader(t *testing.T) {
	r := newRequestIDEngine() // no tracing middleware — the default, disabled-by-default case

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	got := rec.Header().Get("X-Request-ID")
	if !uuidPattern.MatchString(got) {
		t.Errorf("X-Request-ID = %q, want a generated UUID (no trace, no incoming header)", got)
	}
}

func TestRequestID_FallsBackToIncomingHeaderWhenNoTrace(t *testing.T) {
	r := newRequestIDEngine() // no tracing middleware

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "from-load-balancer-123")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "from-load-balancer-123" {
		t.Errorf("X-Request-ID = %q, want the pre-existing behavior preserved: echo the incoming header", got)
	}
}

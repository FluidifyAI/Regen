package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// newRecordingEngine builds a gin.Engine wired to GinMiddleware, backed by a
// tracer provider that records every span in memory rather than exporting —
// real span-creation behavior, no network, no mocked otel internals.
//
// The TracerProvider is injected via otelgin.WithTracerProvider rather than
// otel.SetTracerProvider: the global TracerProvider/propagator in the otel SDK
// is a shared, order-dependent singleton (Get returns the live delegating
// wrapper, not a snapshot — setting it back in cleanup is a self-referential
// no-op), so per-test save/restore of global otel state is unreliable. Passing
// the provider explicitly keeps each test's tracing fully isolated.
func newRecordingEngine(t *testing.T) (*gin.Engine, *tracetest.SpanRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	r := gin.New()
	r.Use(GinMiddleware(otelgin.WithTracerProvider(tp)))
	return r, sr
}

func TestGinMiddleware_ExtractsW3CTraceparent(t *testing.T) {
	r, _ := newRecordingEngine(t)

	var gotTraceID trace.TraceID
	r.GET("/widgets/:id", func(c *gin.Context) {
		gotTraceID = trace.SpanContextFromContext(c.Request.Context()).TraceID()
		c.Status(http.StatusOK)
	})

	const incomingTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	req := httptest.NewRequest(http.MethodGet, "/widgets/abc123", nil)
	req.Header.Set("traceparent", "00-"+incomingTraceID+"-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if got := gotTraceID.String(); got != incomingTraceID {
		t.Errorf("span continued with TraceID %q, want the propagated %q", got, incomingTraceID)
	}
}

func TestGinMiddleware_SpanNameUsesRouteTemplateNotRawPath(t *testing.T) {
	r, sr := newRecordingEngine(t)
	r.GET("/widgets/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/widgets/abc123", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected exactly 1 recorded span, got %d", len(spans))
	}
	if name := spans[0].Name(); name != "/widgets/:id" {
		t.Errorf("span name = %q, want the route template \"/widgets/:id\" (never the raw path, which would be unbounded cardinality)", name)
	}
}

func TestGinMiddleware_ExcludesHealthReadyMetrics(t *testing.T) {
	r, sr := newRecordingEngine(t)
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/ready", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/incidents", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, path := range []string{"/health", "/ready", "/metrics", "/api/v1/incidents"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected exactly 1 recorded span (only the non-excluded route), got %d: %v", len(spans), spanNames(spans))
	}
	if spans[0].Name() != "/api/v1/incidents" {
		t.Errorf("the recorded span should be for /api/v1/incidents, got %q", spans[0].Name())
	}
}

func TestGinMiddleware_TracesWebhookRoutes(t *testing.T) {
	r, sr := newRecordingEngine(t)
	r.POST("/api/v1/webhooks/generic", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/generic", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected webhook route to be traced, got %d spans", len(spans))
	}
	if spans[0].Name() != "/api/v1/webhooks/generic" {
		t.Errorf("span name = %q, want \"/api/v1/webhooks/generic\"", spans[0].Name())
	}
}

func spanNames(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name()
	}
	return names
}

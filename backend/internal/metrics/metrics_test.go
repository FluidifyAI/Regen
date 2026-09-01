package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// histogramWrite pulls the raw proto out of one HistogramVec label
// combination, so tests can inspect bucket counts and exemplars directly —
// the metric objects created by promauto in this package are process-global
// singletons shared across the whole test binary, so tests use a fresh,
// otherwise-unused label combination (a random-looking path per test) to
// avoid cross-test interference instead of resetting the registry.
func histogramWrite(t *testing.T, labels ...string) *dto.Metric {
	t.Helper()
	obs := httpRequestDuration.WithLabelValues(labels...)
	h, ok := obs.(interface{ Write(*dto.Metric) error })
	if !ok {
		t.Fatal("WithLabelValues did not return a Write-able histogram")
	}
	var m dto.Metric
	if err := h.Write(&m); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return &m
}

func firstExemplar(m *dto.Metric) *dto.Exemplar {
	for _, b := range m.GetHistogram().GetBucket() {
		if b.Exemplar != nil {
			return b.Exemplar
		}
	}
	return nil
}

func TestMiddleware_UnmatchedRoute_LabelsAsUnmatchedNotRawPath(t *testing.T) {
	router := gin.New()
	router.Use(Middleware())
	// No routes registered — every request 404s via gin's NoRoute default.

	rawPath := "/reg13-cardinality-probe/attacker-supplied/12345"
	req := httptest.NewRequest(http.MethodGet, rawPath, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// The raw incoming path must never become a label value — unbounded
	// cardinality (REG-13's audit finding: every distinct 404'd URL, including
	// ones an attacker controls, would otherwise mint a new time series
	// forever). A fixed sentinel stands in for it instead.
	m := histogramWrite(t, http.MethodGet, "unmatched", "404")
	if m.GetHistogram().GetSampleCount() != 1 {
		t.Errorf("http_request_duration_seconds{method=GET,path=unmatched,status=404} count = %d, want 1",
			m.GetHistogram().GetSampleCount())
	}

	rawM := histogramWrite(t, http.MethodGet, rawPath, "404")
	if rawM.GetHistogram().GetSampleCount() != 0 {
		t.Errorf("raw incoming path must never appear as its own label value; found %d observation(s) under it",
			rawM.GetHistogram().GetSampleCount())
	}
}

func TestMiddleware_AttachesTraceExemplar_WhenSpanValid(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx, span := tp.Tracer("test").Start(c.Request.Context(), "test-span")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		span.End()
	})
	router.Use(Middleware())
	route := "/reg13-exemplar-probe"
	router.GET(route, func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, route, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	m := histogramWrite(t, http.MethodGet, route, "200")
	if firstExemplar(m) == nil {
		t.Error("expected an exemplar on http_request_duration_seconds when a valid span is present")
	}
}

func TestMiddleware_NoExemplar_WhenNoSpan(t *testing.T) {
	router := gin.New()
	router.Use(Middleware())
	route := "/reg13-no-span-probe"
	router.GET(route, func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, route, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	m := histogramWrite(t, http.MethodGet, route, "200")
	if firstExemplar(m) != nil {
		t.Error("expected no exemplar with no span in the request context")
	}
	if m.GetHistogram().GetSampleCount() != 1 {
		t.Errorf("SampleCount = %d, want 1 — the observation itself must still happen without a span",
			m.GetHistogram().GetSampleCount())
	}
}

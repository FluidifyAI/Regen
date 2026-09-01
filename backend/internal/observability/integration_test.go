package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestRequestLogsShareTraceIDWithSpan is the end-to-end proof of REG-8's own
// acceptance criterion: "Verify a request produces log lines and a trace
// sharing the same trace_id." It wires the real pieces together — GinMiddleware
// (REG-7) opening the span, ContextHandler (this ticket) reading it back out —
// exactly as they're wired in serve.go, without waiting on the full 150-site
// mechanical migration to be able to demonstrate the mechanism works.
func TestRequestLogsShareTraceIDWithSpan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	var logBuf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&logBuf, nil)))

	r := gin.New()
	r.Use(GinMiddleware(otelgin.WithTracerProvider(tp)))
	r.GET("/widgets/:id", func(c *gin.Context) {
		logger.InfoContext(c.Request.Context(), "handling widget request")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/widgets/abc123", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected exactly 1 recorded span, got %d", len(spans))
	}
	wantTraceID := spans[0].SpanContext().TraceID().String()

	var logEntry map[string]string
	if err := json.Unmarshal(logBuf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v\noutput: %s", err, logBuf.String())
	}

	gotTraceID, ok := logEntry["trace_id"]
	if !ok {
		t.Fatalf("log line has no trace_id attribute: %s", logBuf.String())
	}
	if gotTraceID != wantTraceID {
		t.Errorf("log trace_id = %q, span trace_id = %q — want them to match", gotTraceID, wantTraceID)
	}
	if gotTraceID == (trace.TraceID{}).String() {
		t.Error("trace_id is the zero value — the span didn't actually propagate into the log")
	}
}

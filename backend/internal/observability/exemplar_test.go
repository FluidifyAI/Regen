package observability

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func histogramExemplar(t *testing.T, h prometheus.Histogram) *dto.Exemplar {
	t.Helper()
	var m dto.Metric
	if err := h.Write(&m); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buckets := m.GetHistogram().GetBucket()
	for _, b := range buckets {
		if b.Exemplar != nil {
			return b.Exemplar
		}
	}
	return nil
}

func exemplarLabel(e *dto.Exemplar, key string) (string, bool) {
	for _, l := range e.GetLabel() {
		if l.GetName() == key {
			return l.GetValue(), true
		}
	}
	return "", false
}

func TestObserveWithTraceExemplar_AttachesTraceIDWhenSpanValid(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	traceID := span.SpanContext().TraceID().String()

	h := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_hist_1", Buckets: prometheus.DefBuckets})
	ObserveWithTraceExemplar(ctx, h, 0.05)
	span.End()

	ex := histogramExemplar(t, h)
	if ex == nil {
		t.Fatal("expected an exemplar to be attached, got none")
	}
	got, ok := exemplarLabel(ex, "trace_id")
	if !ok || got != traceID {
		t.Errorf("exemplar trace_id = %q, ok=%v, want %q", got, ok, traceID)
	}
}

func TestObserveWithTraceExemplar_NoExemplarWhenNoValidSpan(t *testing.T) {
	h := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_hist_2", Buckets: prometheus.DefBuckets})
	ObserveWithTraceExemplar(context.Background(), h, 0.05)

	if ex := histogramExemplar(t, h); ex != nil {
		t.Errorf("expected no exemplar with no valid span in ctx, got %v", ex)
	}

	// The observation itself must still have been recorded.
	var m dto.Metric
	h.Write(&m)
	if m.GetHistogram().GetSampleCount() != 1 {
		t.Errorf("SampleCount = %d, want 1 (Observe must still happen without a valid span)", m.GetHistogram().GetSampleCount())
	}
}

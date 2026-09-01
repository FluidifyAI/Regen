package observability

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
)

// ObserveWithTraceExemplar records value on obs, attaching the current span's
// trace ID as a Prometheus exemplar when ctx carries one — the bridge that
// turns "this histogram bucket has a latency spike" into "here is an example
// trace of one of the slow requests in it" (REG-13).
//
// Falls back to a plain Observe when ctx has no valid span (tracing
// disabled, or no span was ever started) — the observation is never lost,
// only the exemplar is omitted. obs is typed as the plain prometheus.Observer
// interface so callers don't need to know or care whether their histogram
// happens to support exemplars; classic bucketed histograms from
// promauto.NewHistogram(Vec) always do (see prometheus.NewHistogram's own
// doc comment), so in practice this always attaches one when a span exists.
func ObserveWithTraceExemplar(ctx context.Context, obs prometheus.Observer, value float64) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		obs.Observe(value)
		return
	}

	eo, ok := obs.(prometheus.ExemplarObserver)
	if !ok {
		obs.Observe(value)
		return
	}

	eo.ObserveWithExemplar(value, prometheus.Labels{"trace_id": sc.TraceID().String()})
}

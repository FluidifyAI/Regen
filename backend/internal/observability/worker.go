package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns the shared tracer for hand-instrumented spans outside the
// otelgin/gorm/redisotel integrations — same instrumentation scope name
// ("regen") as those, so all of Regen's spans group under one scope.
func Tracer() trace.Tracer {
	return otel.Tracer("regen")
}

// StartWorkerTick opens a fresh root span for one iteration of a periodic
// background worker's loop.
//
// Deliberately rooted at context.Background(), not the worker's long-lived
// lifecycle context (the ctx passed to Run): that context spans the whole
// process, and parenting every tick under it would produce one unbounded
// "trace" per worker for the server's entire lifetime instead of one span
// per iteration — the wrong shape for "what happened on this particular
// tick" queries.
//
// tracer is a parameter rather than always using the package-global Tracer()
// so tests can inject an isolated tracer instead of mutating global otel
// state (see REG-7's finding: the global TracerProvider is a shared,
// order-dependent singleton, unsafe to save/restore per-test). Production
// callers pass observability.Tracer().
func StartWorkerTick(tracer trace.Tracer, name string) (context.Context, trace.Span) {
	return tracer.Start(context.Background(), name)
}

// EndWorkerTick ends span, recording err as the span's error status when
// non-nil. Callers pair this with StartWorkerTick, typically via defer.
func EndWorkerTick(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

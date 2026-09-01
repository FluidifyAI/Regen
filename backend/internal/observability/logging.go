package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// ContextHandler wraps a slog.Handler, injecting trace_id and span_id
// attributes from the context when a recording span is present. It is a
// transparent passthrough otherwise — no span in context, or tracing
// disabled (a no-op span is never recording) — so there is zero behavior
// change for self-hosters who never configure an OTLP endpoint.
type ContextHandler struct {
	slog.Handler
}

// NewContextHandler wraps handler with trace/span ID injection.
func NewContextHandler(handler slog.Handler) *ContextHandler {
	return &ContextHandler{Handler: handler}
}

// Handle implements slog.Handler.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		sc := span.SpanContext()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs re-wraps the result in a *ContextHandler — without this, a
// logger built via .With(...) would silently lose trace injection, since
// slog.Logger.With ultimately calls the handler's WithAttrs, not Handle,
// to produce the derived handler it then uses.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup re-wraps for the same reason as WithAttrs.
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name)}
}

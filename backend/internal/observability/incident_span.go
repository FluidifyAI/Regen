package observability

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SetIncidentID tags ctx's current span with incident.id — the attribute
// REG-15's runbook depends on to find a trace from an incident ID. Call it
// wherever a specific incident is loaded, created, or mutated and a ctx
// carrying the request/operation's real span is available.
func SetIncidentID(ctx context.Context, id uuid.UUID) {
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("incident.id", id.String()))
}

// MarkIncidentCreated tags ctx's current span with incident.created=true —
// a coarser signal than incident.id, purpose-built for the collector's
// tail-sampling policy (REG-15) to keep 100% of traces that create an
// incident, matched by attribute presence/value rather than needing to
// enumerate every possible incident ID.
func MarkIncidentCreated(ctx context.Context) {
	trace.SpanFromContext(ctx).SetAttributes(attribute.Bool("incident.created", true))
}

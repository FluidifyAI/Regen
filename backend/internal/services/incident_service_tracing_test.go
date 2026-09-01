package services

// Tests for REG-15: incident.id / incident.created span tagging on the
// alert-triggered incident-creation path. CreateIncidentFromAlert and
// CreateIncidentFromAlertWithGrouping already take a real ctx (unlike most
// of IncidentService's other methods — see REG-157's documented remaining
// scope), so they can tag the caller's actual span directly rather than
// needing the handler-layer workaround REG-15 uses elsewhere.

import (
	"context"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func firstEndedSpanAttr(sr *tracetest.SpanRecorder, key string) (string, bool) {
	for _, s := range sr.Ended() {
		for _, a := range s.Attributes() {
			if string(a.Key) == key {
				return a.Value.Emit(), true
			}
		}
	}
	return "", false
}

func TestCreateIncidentFromAlert_TagsSpanWithIncidentIDAndCreated(t *testing.T) {
	db, cleanup := setupPushTestDB(t)
	defer cleanup()

	svc := buildTestIncidentService(db, nil)

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer("test").Start(context.Background(), "webhook.generic")
	alert := makePushTestAlert()
	require.NoError(t, repository.NewAlertRepository(db).Create(ctx, alert))

	incident, err := svc.CreateIncidentFromAlert(ctx, alert, false)
	require.NoError(t, err)
	span.End()

	got, ok := firstEndedSpanAttr(sr, "incident.id")
	if !ok || got != incident.ID.String() {
		t.Errorf("incident.id span attribute = %q, ok=%v, want %q", got, ok, incident.ID.String())
	}
	createdGot, createdOk := firstEndedSpanAttr(sr, "incident.created")
	if !createdOk || createdGot != "true" {
		t.Errorf("incident.created span attribute = %q, ok=%v, want true", createdGot, createdOk)
	}
}

// CreateIncidentFromAlertWithGrouping gets the identical wiring (see
// incident_service.go) but has no dedicated test here: it uses
// pg_advisory_xact_lock, a Postgres-only primitive setupPushTestDB's SQLite
// test database doesn't support, unrelated to REG-15. Covered instead by
// code review — the change is a two-line copy of the block proven by
// TestCreateIncidentFromAlert_TagsSpanWithIncidentIDAndCreated above.

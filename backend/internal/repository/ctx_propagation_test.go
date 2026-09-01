package repository_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	gormtracing "gorm.io/plugin/opentelemetry/tracing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This file proves the REG-157 fix for the incident-create path: repository
// methods that now accept a context.Context actually call db.WithContext(ctx),
// so the gorm tracing plugin registered in REG-9 produces a real child span —
// not just that the method compiles with a new parameter.

var ctxPropagationDBCounter uint64

// setupCtxPropagationTestDB creates an isolated in-memory SQLite database
// with the minimal schema needed for Create/GetByID/Update/LinkAlert/
// GetByExternalID on the incident-create path, and registers the real gorm
// tracing plugin (same as production, via observability.GORMTracingOptions
// in internal/database/database.go) so span assertions exercise the actual
// mechanism REG-9 shipped.
func setupCtxPropagationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	n := atomic.AddUint64(&ctxPropagationDBCounter, 1)
	dsn := fmt.Sprintf("file:ctxproptestdb%d?mode=memory&cache=shared", n)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	stmts := []string{
		`CREATE TABLE incidents (
			id TEXT PRIMARY KEY,
			incident_number INTEGER,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'triggered',
			severity TEXT NOT NULL DEFAULT 'medium',
			summary TEXT,
			slack_channel_id TEXT,
			slack_channel_name TEXT,
			slack_message_ts TEXT,
			teams_channel_id TEXT,
			teams_channel_name TEXT,
			teams_conversation_id TEXT,
			teams_activity_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			triggered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			acknowledged_at DATETIME,
			resolved_at DATETIME,
			created_by_type TEXT NOT NULL,
			created_by_id TEXT NOT NULL DEFAULT '',
			commander_id TEXT,
			labels TEXT DEFAULT '{}',
			custom_fields TEXT DEFAULT '{}',
			group_key TEXT,
			ai_enabled INTEGER NOT NULL DEFAULT 1,
			ai_summary TEXT,
			ai_summary_generated_at DATETIME
		)`,
		`CREATE TABLE alerts (
			id TEXT PRIMARY KEY,
			external_id TEXT NOT NULL,
			source TEXT NOT NULL,
			fingerprint TEXT,
			status TEXT NOT NULL DEFAULT 'firing',
			severity TEXT NOT NULL DEFAULT 'info',
			title TEXT NOT NULL,
			description TEXT,
			labels TEXT DEFAULT '{}',
			annotations TEXT DEFAULT '{}',
			raw_payload TEXT,
			started_at DATETIME NOT NULL,
			ended_at DATETIME,
			received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			escalation_policy_id TEXT,
			acknowledgment_status TEXT NOT NULL DEFAULT 'pending'
		)`,
		`CREATE TABLE timeline_entries (
			id TEXT PRIMARY KEY,
			incident_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			type TEXT NOT NULL,
			actor_type TEXT NOT NULL,
			actor_id TEXT,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE incident_alerts (
			incident_id TEXT NOT NULL,
			alert_id TEXT NOT NULL,
			linked_by_type TEXT NOT NULL,
			linked_by_id TEXT NOT NULL,
			PRIMARY KEY (incident_id, alert_id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("exec DDL: %v\nSQL: %s", err, stmt)
		}
	}

	return db
}

// tracedDB wires the real gorm tracing plugin against an isolated
// TracerProvider, mirroring internal/database/database.go's production
// wiring without touching global otel state (see REG-7's finding).
func tracedDB(t *testing.T, db *gorm.DB) (*gorm.DB, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	if err := db.Use(gormtracing.NewPlugin(
		gormtracing.WithoutQueryVariables(),
		gormtracing.WithTracerProvider(tp),
	)); err != nil {
		t.Fatalf("register tracing plugin: %v", err)
	}
	return db, sr
}

func TestIncidentRepository_Create_ProducesChildSpanUnderCallersTrace(t *testing.T) {
	db := setupCtxPropagationTestDB(t)
	db, sr := tracedDB(t, db)
	repo := repository.NewIncidentRepository(db)

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())
	ctx, parentSpan := tp.Tracer("test").Start(context.Background(), "webhook-request")

	incident := &models.Incident{
		ID:            uuid.New(),
		Title:         "test",
		Slug:          "test-" + uuid.New().String()[:8],
		CreatedByType: "system",
		TriggeredAt:   time.Now(),
	}
	if err := repo.Create(ctx, incident); err != nil {
		t.Fatalf("Create: %v", err)
	}
	parentSpan.End()

	spans := sr.Ended()
	var dbSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Parent().SpanID() == parentSpan.SpanContext().SpanID() {
			dbSpan = s
		}
	}
	if dbSpan == nil {
		t.Fatalf("no span was recorded as a child of the parent span — Create did not thread ctx into db.WithContext; spans seen: %d", len(spans))
	}
	if dbSpan.SpanContext().TraceID() != parentSpan.SpanContext().TraceID() {
		t.Error("incident Create span has a different trace ID than the caller — not actually correlated")
	}
}

func TestAlertRepository_Create_ProducesChildSpanUnderCallersTrace(t *testing.T) {
	db := setupCtxPropagationTestDB(t)
	db, sr := tracedDB(t, db)
	repo := repository.NewAlertRepository(db)

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())
	ctx, parentSpan := tp.Tracer("test").Start(context.Background(), "webhook-request")

	alert := &models.Alert{
		ID:         uuid.New(),
		ExternalID: "ext-1",
		Source:     "prometheus",
		Title:      "test alert",
		StartedAt:  time.Now(),
	}
	if err := repo.Create(ctx, alert); err != nil {
		t.Fatalf("Create: %v", err)
	}
	parentSpan.End()

	spans := sr.Ended()
	var dbSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Parent().SpanID() == parentSpan.SpanContext().SpanID() {
			dbSpan = s
		}
	}
	if dbSpan == nil {
		t.Fatalf("no span was recorded as a child of the parent span — Create did not thread ctx into db.WithContext; spans seen: %d", len(spans))
	}
	if dbSpan.SpanContext().TraceID() != parentSpan.SpanContext().TraceID() {
		t.Error("alert Create span has a different trace ID than the caller — not actually correlated")
	}
}

func TestTimelineRepository_Create_ProducesChildSpanUnderCallersTrace(t *testing.T) {
	db := setupCtxPropagationTestDB(t)
	db, sr := tracedDB(t, db)
	repo := repository.NewTimelineRepository(db)

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())
	ctx, parentSpan := tp.Tracer("test").Start(context.Background(), "webhook-request")

	entry := &models.TimelineEntry{
		ID:         uuid.New(),
		IncidentID: uuid.New(),
		Timestamp:  time.Now(),
		Type:       "incident_created",
		ActorType:  "system",
		Content:    models.JSONB{"k": "v"},
	}
	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("Create: %v", err)
	}
	parentSpan.End()

	spans := sr.Ended()
	var dbSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Parent().SpanID() == parentSpan.SpanContext().SpanID() {
			dbSpan = s
		}
	}
	if dbSpan == nil {
		t.Fatalf("no span was recorded as a child of the parent span — Create did not thread ctx into db.WithContext; spans seen: %d", len(spans))
	}
	if dbSpan.SpanContext().TraceID() != parentSpan.SpanContext().TraceID() {
		t.Error("timeline Create span has a different trace ID than the caller — not actually correlated")
	}
}

// TestRepository_MethodsOnIncidentCreatePath_AllThreadContext is a completeness
// check, not a mechanism proof (the three tests above already proved the
// mechanism): confirms every method REG-157 scoped for this slice compiles
// with a ctx parameter. A method missing from this list would fail to compile
// this file at all, but this test also documents the exact intended surface
// so a future reviewer can see the scope decision in one place.
func TestRepository_MethodsOnIncidentCreatePath_AllThreadContext(t *testing.T) {
	db := setupCtxPropagationTestDB(t)
	ctx := context.Background()

	incidentRepo := repository.NewIncidentRepository(db)
	alertRepo := repository.NewAlertRepository(db)
	timelineRepo := repository.NewTimelineRepository(db)

	incident := &models.Incident{
		ID: uuid.New(), Title: "t", Slug: "s-" + uuid.New().String()[:8],
		CreatedByType: "system", TriggeredAt: time.Now(),
	}
	if err := incidentRepo.Create(ctx, incident); err != nil {
		t.Fatalf("IncidentRepository.Create: %v", err)
	}
	if _, err := incidentRepo.GetByID(ctx, incident.ID); err != nil {
		t.Fatalf("IncidentRepository.GetByID: %v", err)
	}
	incident.Summary = "updated"
	if err := incidentRepo.Update(ctx, incident); err != nil {
		t.Fatalf("IncidentRepository.Update: %v", err)
	}

	alert := &models.Alert{
		ID: uuid.New(), ExternalID: "e1", Source: "prometheus", Title: "a", StartedAt: time.Now(),
	}
	if err := alertRepo.Create(ctx, alert); err != nil {
		t.Fatalf("AlertRepository.Create: %v", err)
	}
	if _, err := alertRepo.GetByExternalID(ctx, "prometheus", "e1"); err != nil {
		t.Fatalf("AlertRepository.GetByExternalID: %v", err)
	}
	alert.Title = "updated"
	if err := alertRepo.Update(ctx, alert); err != nil {
		t.Fatalf("AlertRepository.Update: %v", err)
	}

	if err := incidentRepo.LinkAlert(ctx, incident.ID, alert.ID, "system", "test"); err != nil {
		t.Fatalf("IncidentRepository.LinkAlert: %v", err)
	}

	entry := &models.TimelineEntry{
		ID: uuid.New(), IncidentID: incident.ID, Timestamp: time.Now(),
		Type: "incident_created", ActorType: "system", Content: models.JSONB{},
	}
	if err := timelineRepo.Create(ctx, entry); err != nil {
		t.Fatalf("TimelineRepository.Create: %v", err)
	}
}

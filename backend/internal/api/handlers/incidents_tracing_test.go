package handlers

// Tests for REG-15: incident.id / incident.created span tagging on the core
// incident HTTP handlers. Kept separate from incidents_test.go, which uses a
// real DB-backed IncidentService and a bare gin.New() router with no tracing
// middleware — neither lets a test observe span attributes. These tests use
// a minimal in-memory IncidentService double and a real TracerProvider +
// SpanRecorder instead, so they can assert on the actual span REG-15
// depends on.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/llm"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakeIncidentServiceForTracing implements services.IncidentService with
// only GetIncident/CreateIncident/UpdateIncident behaviorally configurable;
// every other method is an unused no-op. Scoped to this file only.
type fakeIncidentServiceForTracing struct {
	incident  *models.Incident // returned by GetIncident and UpdateIncident
	getErr    error
	created   *models.Incident // returned by CreateIncident
	createErr error
	updateErr error
}

func (f *fakeIncidentServiceForTracing) CreateIncidentFromAlert(context.Context, *models.Alert, bool) (*models.Incident, error) {
	return nil, nil
}
func (f *fakeIncidentServiceForTracing) CreateIncidentFromAlertWithGrouping(context.Context, *models.Alert, string, bool) (*models.Incident, error) {
	return nil, nil
}
func (f *fakeIncidentServiceForTracing) LinkAlertToExistingIncident(context.Context, *models.Alert, uuid.UUID) error {
	return nil
}
func (f *fakeIncidentServiceForTracing) ShouldCreateIncident(models.AlertSeverity) bool { return false }
func (f *fakeIncidentServiceForTracing) CreateSlackChannelForIncident(*models.Incident, []models.Alert) error {
	return nil
}
func (f *fakeIncidentServiceForTracing) ListIncidents(repository.IncidentFilters, repository.Pagination) ([]models.Incident, int64, error) {
	return nil, 0, nil
}
func (f *fakeIncidentServiceForTracing) GetIncident(uuid.UUID, int) (*models.Incident, error) {
	return f.incident, f.getErr
}
func (f *fakeIncidentServiceForTracing) GetIncidentBySlackChannelID(string) (*models.Incident, error) {
	return nil, nil
}
func (f *fakeIncidentServiceForTracing) GetIncidentBySlackMessageTS(string) (*models.Incident, error) {
	return nil, nil
}
func (f *fakeIncidentServiceForTracing) CreateIncident(*services.CreateIncidentParams) (*models.Incident, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.created, nil
}
func (f *fakeIncidentServiceForTracing) UpdateIncident(uuid.UUID, *services.UpdateIncidentParams) (*models.Incident, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.incident, nil
}
func (f *fakeIncidentServiceForTracing) AcknowledgeIncident(uuid.UUID, string, string) error {
	return nil
}
func (f *fakeIncidentServiceForTracing) ResolveIncident(uuid.UUID, string, string) error { return nil }
func (f *fakeIncidentServiceForTracing) GetIncidentAlerts(uuid.UUID) ([]models.Alert, error) {
	return nil, nil
}
func (f *fakeIncidentServiceForTracing) GetIncidentTimeline(uuid.UUID, repository.Pagination) ([]models.TimelineEntry, int64, error) {
	return nil, 0, nil
}
func (f *fakeIncidentServiceForTracing) CreateTimelineEntry(*services.CreateTimelineEntryParams) (*models.TimelineEntry, error) {
	return nil, nil
}
func (f *fakeIncidentServiceForTracing) UpdateIncidentStatus(uuid.UUID, models.IncidentStatus, string, string) error {
	return nil
}
func (f *fakeIncidentServiceForTracing) PostStatusUpdateToSlack(*models.Incident, models.IncidentStatus, models.IncidentStatus) error {
	return nil
}
func (f *fakeIncidentServiceForTracing) GenerateAISummary(*models.Incident) (string, llm.Usage, error) {
	return "", llm.Usage{}, nil
}
func (f *fakeIncidentServiceForTracing) GenerateHandoffDigest(*models.Incident) (string, llm.Usage, error) {
	return "", llm.Usage{}, nil
}

var _ services.IncidentService = &fakeIncidentServiceForTracing{}

// newTracedGinRouter builds a gin router whose requests carry a real span
// (via an injected TracerProvider), and returns the SpanRecorder to assert
// against — otelgin's own middleware isn't needed here, just something that
// puts a real span on c.Request.Context() the way otelgin.Middleware does in
// production.
func newTracedGinRouter(t *testing.T) (*gin.Engine, *tracetest.SpanRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { tp.Shutdown(context.Background()) })

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx, span := tp.Tracer("test").Start(c.Request.Context(), "http.request")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		span.End()
	})
	return router, sr
}

func firstSpanAttr(sr *tracetest.SpanRecorder, key string) (string, bool) {
	for _, s := range sr.Ended() {
		for _, a := range s.Attributes() {
			if string(a.Key) == key {
				return a.Value.Emit(), true
			}
		}
	}
	return "", false
}

func TestGetIncident_TagsSpanWithIncidentID(t *testing.T) {
	id := uuid.New()
	svc := &fakeIncidentServiceForTracing{incident: &models.Incident{ID: id, IncidentNumber: 1}}
	router, sr := newTracedGinRouter(t)
	router.GET("/api/v1/incidents/:id", GetIncident(svc, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/"+id.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got, ok := firstSpanAttr(sr, "incident.id")
	if !ok || got != id.String() {
		t.Errorf("incident.id span attribute = %q, ok=%v, want %q", got, ok, id.String())
	}
}

func TestCreateIncident_TagsSpanWithIncidentIDAndCreated(t *testing.T) {
	id := uuid.New()
	svc := &fakeIncidentServiceForTracing{created: &models.Incident{ID: id, Severity: models.IncidentSeverityHigh}}
	router, sr := newTracedGinRouter(t)
	router.POST("/api/v1/incidents", CreateIncident(svc))

	body := `{"title":"db down","severity":"high"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	got, ok := firstSpanAttr(sr, "incident.id")
	if !ok || got != id.String() {
		t.Errorf("incident.id span attribute = %q, ok=%v, want %q", got, ok, id.String())
	}
	createdGot, createdOk := firstSpanAttr(sr, "incident.created")
	if !createdOk || createdGot != "true" {
		t.Errorf("incident.created span attribute = %q, ok=%v, want true", createdGot, createdOk)
	}
}

func TestUpdateIncident_TagsSpanWithIncidentID(t *testing.T) {
	id := uuid.New()
	svc := &fakeIncidentServiceForTracing{
		incident: &models.Incident{ID: id, IncidentNumber: 1, Status: models.IncidentStatusTriggered},
	}
	router, sr := newTracedGinRouter(t)
	router.PATCH("/api/v1/incidents/:id", UpdateIncident(svc))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/incidents/"+id.String(), strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got, ok := firstSpanAttr(sr, "incident.id")
	if !ok || got != id.String() {
		t.Errorf("incident.id span attribute = %q, ok=%v, want %q", got, ok, id.String())
	}
}

package services

// Shared test helpers for alert service pipeline tests (OI-127+).
// These mocks are minimal — they only stub the methods exercised by
// ProcessNormalizedAlerts.

import (
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/models/webhooks"
	"github.com/openincident/openincident/internal/repository"
)

// ── mockAlertRepository ───────────────────────────────────────────────────────

type mockAlertRepository struct {
	// If existingAlert is set, GetByExternalID returns it (simulating a duplicate).
	existingAlert *models.Alert
}

func (m *mockAlertRepository) Create(alert *models.Alert) error { return nil }
func (m *mockAlertRepository) GetByID(id uuid.UUID) (*models.Alert, error) {
	return nil, &repository.NotFoundError{Resource: "alert", ID: id.String()}
}
func (m *mockAlertRepository) GetByExternalID(source, externalID string) (*models.Alert, error) {
	if m.existingAlert != nil &&
		m.existingAlert.Source == source &&
		m.existingAlert.ExternalID == externalID {
		return m.existingAlert, nil
	}
	return nil, &repository.NotFoundError{Resource: "alert", ID: externalID}
}
func (m *mockAlertRepository) List(filters repository.AlertFilters, pagination repository.Pagination) ([]models.Alert, int64, error) {
	return nil, 0, nil
}
func (m *mockAlertRepository) Update(alert *models.Alert) error { return nil }

var _ repository.AlertRepository = &mockAlertRepository{}

// ── mockIncidentService ───────────────────────────────────────────────────────

type mockIncidentService struct {
	shouldCreate bool
}

func (m *mockIncidentService) ShouldCreateIncident(severity models.AlertSeverity) bool {
	return m.shouldCreate
}
func (m *mockIncidentService) CreateIncidentFromAlert(alert *models.Alert) (*models.Incident, error) {
	return &models.Incident{ID: uuid.New()}, nil
}
func (m *mockIncidentService) CreateIncidentFromAlertWithGrouping(alert *models.Alert, groupKey string) (*models.Incident, error) {
	return &models.Incident{ID: uuid.New()}, nil
}
func (m *mockIncidentService) LinkAlertToExistingIncident(alert *models.Alert, incidentID uuid.UUID) error {
	return nil
}
func (m *mockIncidentService) CreateSlackChannelForIncident(incident *models.Incident, alerts []models.Alert) error {
	return nil
}
func (m *mockIncidentService) ListIncidents(filters repository.IncidentFilters, pagination repository.Pagination) ([]models.Incident, int64, error) {
	return nil, 0, nil
}
func (m *mockIncidentService) GetIncident(id uuid.UUID, number int) (*models.Incident, error) {
	return nil, &repository.NotFoundError{Resource: "incident", ID: id.String()}
}
func (m *mockIncidentService) GetIncidentBySlackChannelID(channelID string) (*models.Incident, error) {
	return nil, nil
}
func (m *mockIncidentService) CreateIncident(params *CreateIncidentParams) (*models.Incident, error) {
	return &models.Incident{ID: uuid.New()}, nil
}
func (m *mockIncidentService) UpdateIncident(id uuid.UUID, params *UpdateIncidentParams) (*models.Incident, error) {
	return nil, nil
}
func (m *mockIncidentService) GetIncidentAlerts(incidentID uuid.UUID) ([]models.Alert, error) {
	return nil, nil
}
func (m *mockIncidentService) GetIncidentTimeline(incidentID uuid.UUID, pagination repository.Pagination) ([]models.TimelineEntry, int64, error) {
	return nil, 0, nil
}
func (m *mockIncidentService) CreateTimelineEntry(params *CreateTimelineEntryParams) (*models.TimelineEntry, error) {
	return nil, nil
}
func (m *mockIncidentService) PostStatusUpdateToSlack(incident *models.Incident, previousStatus, newStatus models.IncidentStatus) error {
	return nil
}
func (m *mockIncidentService) GenerateAISummary(_ *models.Incident) (string, error) {
	return "", nil
}
func (m *mockIncidentService) GenerateHandoffDigest(_ *models.Incident) (string, error) {
	return "", nil
}
func (m *mockIncidentService) AcknowledgeIncident(_ uuid.UUID, _, _ string) error { return nil }
func (m *mockIncidentService) ResolveIncident(_ uuid.UUID, _, _ string) error     { return nil }

var _ IncidentService = &mockIncidentService{}

// ── makeNormalizedAlert ───────────────────────────────────────────────────────

func makeNormalizedAlert(source, severity string) webhooks.NormalizedAlert {
	return webhooks.NormalizedAlert{
		ExternalID: uuid.New().String(),
		Source:     source,
		Status:     "firing",
		Severity:   severity,
		Title:      "Test alert",
		StartedAt:  time.Now(),
	}
}

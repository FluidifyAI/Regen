package services

import (
	"context"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/models/webhooks"
	"github.com/stretchr/testify/require"
)

type severityIncidentService struct {
	mockIncidentService
	captured *models.Alert
}

func (s *severityIncidentService) ShouldCreateIncident(v models.AlertSeverity) bool {
	return (&incidentService{}).ShouldCreateIncident(v)
}
func (s *severityIncidentService) CreateIncidentFromAlert(ctx context.Context, a *models.Alert, ai bool) (*models.Incident, error) {
	s.captured = a
	return &models.Incident{Severity: incidentSeverityForAlert(a)}, nil
}

func TestGenericExplicitIncidentSeverity(t *testing.T) {
	for _, level := range []string{"critical", "high", "medium", "low"} {
		t.Run(level, func(t *testing.T) {
			for _, duplicate := range []bool{false, true} {
				repo := &mockAlertRepository{}
				if duplicate {
					repo.existingAlert = &models.Alert{Source: "generic", ExternalID: "test", Status: models.AlertStatusFiring}
				}
				incidents := &severityIncidentService{}
				svc := NewAlertService(repo, incidents)
				alerts, err := (&webhooks.GenericProvider{}).ParsePayload([]byte(`{"title":"Test", "external_id":"test", "severity":"info", "incident_severity":"` + level + `"}`))
				require.NoError(t, err)
				result, err := svc.ProcessNormalizedAlerts(context.Background(), alerts)
				require.NoError(t, err)
				// A duplicate with the mock service does not create a second incident.
				if duplicate {
					require.Zero(t, result.IncidentsCreated)
					continue
				}
				require.Equal(t, 1, result.IncidentsCreated)
				require.NotNil(t, incidents.captured)
				require.Equal(t, models.AlertSeverityInfo, incidents.captured.Severity)
				require.Equal(t, models.IncidentSeverity(level), incidentSeverityForAlert(incidents.captured))
			}
		})
	}
}

func TestLegacyIncidentSeverity(t *testing.T) {
	for _, tc := range []struct {
		severity string
		count    int
		level    models.IncidentSeverity
	}{
		{"info", 0, ""}, {"warning", 1, models.IncidentSeverityHigh}, {"critical", 1, models.IncidentSeverityCritical},
	} {
		t.Run(tc.severity, func(t *testing.T) {
			incidents := &severityIncidentService{}
			svc := NewAlertService(&mockAlertRepository{}, incidents)
			result, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{{Title: "Test", Source: "generic", ExternalID: "test", Status: "firing", Severity: tc.severity}})
			require.NoError(t, err)
			require.Equal(t, tc.count, result.IncidentsCreated)
			if tc.count > 0 {
				require.Equal(t, tc.level, incidentSeverityForAlert(incidents.captured))
			}
		})
	}
}

type severityRoutingEngine struct{ decision RoutingDecision }

func (r severityRoutingEngine) EvaluateAlert(*models.Alert) (*RoutingDecision, error) {
	return &r.decision, nil
}
func (r severityRoutingEngine) RefreshRules() error { return nil }

type severityGroupingEngine struct{}

func (severityGroupingEngine) EvaluateAlert(*models.Alert) (*GroupingDecision, error) {
	return &GroupingDecision{Action: GroupActionCreateNew, GroupKey: "test-group"}, nil
}
func (severityGroupingEngine) RefreshRules() error { return nil }
func (s *severityIncidentService) CreateIncidentFromAlertWithGrouping(ctx context.Context, a *models.Alert, key string, ai bool) (*models.Incident, error) {
	return s.CreateIncidentFromAlert(ctx, a, ai)
}

func TestExplicitIncidentSeverityRoutingAndGrouping(t *testing.T) {
	for _, tc := range []struct {
		name     string
		decision RoutingDecision
		count    int
		severity models.IncidentSeverity
	}{
		{"explicit", RoutingDecision{}, 1, models.IncidentSeverityLow},
		{"suppressed", RoutingDecision{Suppress: true}, 0, ""},
		{"override warning", RoutingDecision{SeverityOverride: "warning"}, 1, models.IncidentSeverityHigh},
		{"override info", RoutingDecision{SeverityOverride: "info"}, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			incidents := &severityIncidentService{}
			svc := NewAlertService(&mockAlertRepository{}, incidents)
			svc.SetRoutingEngine(severityRoutingEngine{tc.decision})
			svc.SetGroupingEngine(severityGroupingEngine{})
			result, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{{Title: "Test", Source: "generic", ExternalID: "test", Status: "firing", Severity: "info", IncidentSeverity: "low"}})
			require.NoError(t, err)
			require.Equal(t, tc.count, result.IncidentsCreated)
			if tc.count > 0 {
				require.Equal(t, tc.severity, incidentSeverityForAlert(incidents.captured))
			}
		})
	}
}

func TestDuplicatePreservesIncidentSeverityRequest(t *testing.T) {
	svc := &alertService{alertRepo: &mockAlertRepository{existingAlert: &models.Alert{Source: "generic", ExternalID: "test"}}}
	alert := &models.Alert{Source: "generic", ExternalID: "test", IncidentSeverity: models.IncidentSeverityMedium}
	_, err := svc.createOrUpdateAlert(context.Background(), alert)
	require.NoError(t, err)
	require.Equal(t, models.IncidentSeverityMedium, alert.IncidentSeverity)
}

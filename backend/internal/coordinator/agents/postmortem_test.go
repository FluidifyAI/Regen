package agents_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/coordinator/agents"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"github.com/stretchr/testify/assert"
)

// --- Fakes ---

type fakeIncidentRepo struct{ incident *models.Incident }

func (f *fakeIncidentRepo) GetByID(id uuid.UUID) (*models.Incident, error) { return f.incident, nil }

type fakePostMortemSvc struct {
	called      bool
	returnError error
	existing    bool
}

func (f *fakePostMortemSvc) GetPostMortem(incidentID uuid.UUID) (*models.PostMortem, error) {
	if f.existing {
		return &models.PostMortem{}, nil
	}
	return nil, &repository.NotFoundError{Resource: "post_mortem", ID: incidentID.String()}
}

func (f *fakePostMortemSvc) GeneratePostMortem(incident *models.Incident, templateID *uuid.UUID, createdByID string) (*models.PostMortem, error) {
	f.called = true
	return &models.PostMortem{ID: uuid.New()}, f.returnError
}

type fakeAISvc struct{ enabled bool }

func (f *fakeAISvc) IsEnabled() bool { return f.enabled }

// --- Tests ---

func TestPostMortemAgent_SkipsWhenAIDisabled(t *testing.T) {
	pmSvc := &fakePostMortemSvc{}
	agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
		AgentUserID:   uuid.New(),
		AISvc:         &fakeAISvc{enabled: false},
		IncidentRepo:  &fakeIncidentRepo{incident: &models.Incident{ID: uuid.New(), Status: "resolved"}},
		PostMortemSvc: pmSvc,
		WaitDuration:  0,
	})
	agent.Handle(context.Background(), uuid.New())
	assert.False(t, pmSvc.called, "should not generate when AI disabled")
}

func TestPostMortemAgent_SkipsWhenPostMortemExists(t *testing.T) {
	pmSvc := &fakePostMortemSvc{existing: true}
	agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
		AgentUserID:   uuid.New(),
		AISvc:         &fakeAISvc{enabled: true},
		IncidentRepo:  &fakeIncidentRepo{incident: &models.Incident{ID: uuid.New(), Status: "resolved"}},
		PostMortemSvc: pmSvc,
		WaitDuration:  0,
	})
	agent.Handle(context.Background(), uuid.New())
	assert.False(t, pmSvc.called, "should not overwrite existing post-mortem")
}

func TestPostMortemAgent_GeneratesWhenAllConditionsMet(t *testing.T) {
	resolvedAt := time.Now().Add(-30 * time.Minute)
	triggeredAt := resolvedAt.Add(-30 * time.Minute)
	incident := &models.Incident{
		ID: uuid.New(), Status: "resolved",
		TriggeredAt: triggeredAt, ResolvedAt: &resolvedAt,
	}
	pmSvc := &fakePostMortemSvc{existing: false}
	agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
		AgentUserID:   uuid.New(),
		AISvc:         &fakeAISvc{enabled: true},
		IncidentRepo:  &fakeIncidentRepo{incident: incident},
		PostMortemSvc: pmSvc,
		WaitDuration:  0,
	})
	agent.Handle(context.Background(), incident.ID)
	assert.True(t, pmSvc.called, "should generate post-mortem")
}

func TestPostMortemAgent_SkipsShortIncidents(t *testing.T) {
	resolvedAt := time.Now().Add(-1 * time.Minute)
	triggeredAt := resolvedAt.Add(-2 * time.Minute) // only 2 minutes — below 5m threshold
	incident := &models.Incident{
		ID: uuid.New(), Status: "resolved",
		TriggeredAt: triggeredAt, ResolvedAt: &resolvedAt,
	}
	pmSvc := &fakePostMortemSvc{existing: false}
	agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
		AgentUserID:   uuid.New(),
		AISvc:         &fakeAISvc{enabled: true},
		IncidentRepo:  &fakeIncidentRepo{incident: incident},
		PostMortemSvc: pmSvc,
		WaitDuration:  0,
	})
	agent.Handle(context.Background(), incident.ID)
	assert.False(t, pmSvc.called, "should skip incidents under 5 minutes")
}

func TestPostMortemAgent_SkipsWhenNotResolved(t *testing.T) {
	// Incident without a ResolvedAt (e.g. race condition or manual call)
	incident := &models.Incident{
		ID:          uuid.New(),
		Status:      "resolved",
		TriggeredAt: time.Now().Add(-1 * time.Hour),
		ResolvedAt:  nil, // no resolved timestamp
	}
	pmSvc := &fakePostMortemSvc{existing: false}
	agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
		AgentUserID:   uuid.New(),
		AISvc:         &fakeAISvc{enabled: true},
		IncidentRepo:  &fakeIncidentRepo{incident: incident},
		PostMortemSvc: pmSvc,
		WaitDuration:  0,
	})
	agent.Handle(context.Background(), incident.ID)
	assert.False(t, pmSvc.called, "should skip incident without resolved timestamp")
}

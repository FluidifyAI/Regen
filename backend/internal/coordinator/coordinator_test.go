package coordinator_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/coordinator"
	"github.com/stretchr/testify/assert"
)

// fakeAgent records whether Handle was called
type fakeAgent struct {
	called     bool
	incidentID uuid.UUID
}

func (f *fakeAgent) Handle(ctx context.Context, incidentID uuid.UUID) {
	f.called = true
	f.incidentID = incidentID
}

func TestCoordinator_RoutesResolvedEvent(t *testing.T) {
	// Integration test — requires Redis. Skip in unit test runs.
	t.Skip("integration test — run with make test-integration")
}

func TestCoordinator_SkipsWhenAIDisabled(t *testing.T) {
	incidentID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"incident_id": incidentID.String(),
		"ai_enabled":  false,
	})

	agent := &fakeAgent{}
	c := coordinator.NewTestCoordinator(agent)
	c.RoutePayload("events:incident.resolved", payload)

	assert.False(t, agent.called, "agent should not be called when ai_enabled=false")
}

func TestCoordinator_CallsAgentWhenAIEnabled(t *testing.T) {
	incidentID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"incident_id": incidentID.String(),
		"ai_enabled":  true,
	})

	agent := &fakeAgent{}
	c := coordinator.NewTestCoordinator(agent)
	c.RoutePayload("events:incident.resolved", payload)

	assert.True(t, agent.called)
	assert.Equal(t, incidentID, agent.incidentID)
}

package coordinator_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/coordinator"
	"github.com/stretchr/testify/assert"
)

// fakeAgent records whether Handle was called
type fakeAgent struct {
	mu         sync.Mutex
	called     bool
	incidentID uuid.UUID
}

func (f *fakeAgent) Handle(ctx context.Context, incidentID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
	c.RoutePayload(context.Background(), "events:incident.resolved", payload)

	agent.mu.Lock()
	calledVal := agent.called
	agent.mu.Unlock()
	assert.False(t, calledVal, "agent should not be called when ai_enabled=false")
}

func TestCoordinator_CallsAgentWhenAIEnabled(t *testing.T) {
	incidentID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"incident_id": incidentID.String(),
		"ai_enabled":  true,
	})

	agent := &fakeAgent{}
	c := coordinator.NewTestCoordinator(agent)
	c.RoutePayload(context.Background(), "events:incident.resolved", payload)

	agent.mu.Lock()
	calledVal := agent.called
	idVal := agent.incidentID
	agent.mu.Unlock()
	assert.True(t, calledVal)
	assert.Equal(t, incidentID, idVal)
}

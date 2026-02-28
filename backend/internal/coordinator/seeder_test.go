package coordinator_test

import (
	"testing"

	"github.com/openincident/openincident/internal/coordinator"
	"github.com/openincident/openincident/internal/database"
	"github.com/openincident/openincident/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedAgents_CreatesPostMortemAgent(t *testing.T) {
	db := database.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	err := coordinator.SeedAgents(userRepo)
	require.NoError(t, err)

	agent, err := userRepo.GetByEmail(coordinator.PostMortemAgentEmail)
	require.NoError(t, err)
	assert.Equal(t, "Post-Mortem Agent", agent.Name)
	assert.Equal(t, "ai", agent.AuthSource)
	assert.NotNil(t, agent.AgentType)
	assert.Equal(t, "postmortem", *agent.AgentType)
	assert.True(t, agent.Active)
}

func TestSeedAgents_IsIdempotent(t *testing.T) {
	db := database.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	// Call twice — should not error or create duplicates
	require.NoError(t, coordinator.SeedAgents(userRepo))
	require.NoError(t, coordinator.SeedAgents(userRepo))

	agents, err := userRepo.ListAgents()
	require.NoError(t, err)
	assert.Len(t, agents, 1)
}

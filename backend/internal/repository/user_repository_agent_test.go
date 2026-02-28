package repository_test

import (
	"testing"

	"github.com/openincident/openincident/internal/database"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_CreateAgent(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewUserRepository(db)

	agentType := "postmortem"
	agent := &models.User{
		Email:      "agent-postmortem@system.internal",
		Name:       "Post-Mortem Agent",
		AuthSource: "ai",
		AgentType:  &agentType,
		Role:       models.UserRoleMember,
		Active:     true,
	}

	err := repo.CreateAgent(agent)
	require.NoError(t, err)
	assert.NotEmpty(t, agent.ID)

	found, err := repo.GetByEmail("agent-postmortem@system.internal")
	require.NoError(t, err)
	assert.Equal(t, "ai", found.AuthSource)
	assert.Equal(t, "postmortem", *found.AgentType)
}

func TestUserRepository_SetActive(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewUserRepository(db)

	agentType := "postmortem"
	agent := &models.User{
		Email: "agent-test@system.internal", Name: "Test Agent",
		AuthSource: "ai", AgentType: &agentType, Role: models.UserRoleMember, Active: true,
	}
	require.NoError(t, repo.CreateAgent(agent))

	require.NoError(t, repo.SetActive(agent.ID, false))
	found, err := repo.GetByID(agent.ID)
	require.NoError(t, err)
	assert.False(t, found.Active)

	require.NoError(t, repo.SetActive(agent.ID, true))
	found, err = repo.GetByID(agent.ID)
	require.NoError(t, err)
	assert.True(t, found.Active)
}

func TestUserRepository_ListAgents(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewUserRepository(db)

	agentType := "postmortem"
	require.NoError(t, repo.CreateAgent(&models.User{
		Email: "agent-pm@system.internal", Name: "Post-Mortem Agent",
		AuthSource: "ai", AgentType: &agentType, Role: models.UserRoleMember, Active: true,
	}))

	agents, err := repo.ListAgents()
	require.NoError(t, err)
	assert.Len(t, agents, 1)
	assert.Equal(t, "ai", agents[0].AuthSource)
}

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/fluidify/regen/internal/database"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentsHandler_List(t *testing.T) {
	db := database.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	agentType := "postmortem"
	require.NoError(t, userRepo.CreateAgent(&models.User{
		Email: "agent-pm@system.internal", Name: "Post-Mortem Agent",
		AuthSource: "ai", AgentType: &agentType, Role: models.UserRoleMember, Active: true,
	}))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAgentsHandler(userRepo)
	r.GET("/api/v1/agents", h.List)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/agents", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body, 1)
	assert.Equal(t, "Post-Mortem Agent", body[0]["name"])
}

func TestAgentsHandler_SetStatus(t *testing.T) {
	db := database.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	agentType := "postmortem"
	agent := &models.User{
		Email: "agent-pm2@system.internal", Name: "Post-Mortem Agent",
		AuthSource: "ai", AgentType: &agentType, Role: models.UserRoleMember, Active: true,
	}
	require.NoError(t, userRepo.CreateAgent(agent))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAgentsHandler(userRepo)
	r.PATCH("/api/v1/agents/:id/status", h.SetStatus)

	body := `{"active": false}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/v1/agents/"+agent.ID.String()+"/status",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	found, err := userRepo.GetByID(agent.ID)
	require.NoError(t, err)
	assert.False(t, found.Active)
}

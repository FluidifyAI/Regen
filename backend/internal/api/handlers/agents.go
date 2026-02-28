package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/repository"
)

// AgentsHandler manages AI agent endpoints.
type AgentsHandler struct {
	userRepo repository.UserRepository
}

// NewAgentsHandler constructs an AgentsHandler backed by the given UserRepository.
func NewAgentsHandler(userRepo repository.UserRepository) *AgentsHandler {
	return &AgentsHandler{userRepo: userRepo}
}

// List returns all AI agent users.
// GET /api/v1/agents
func (h *AgentsHandler) List(c *gin.Context) {
	agents, err := h.userRepo.ListAgents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agents"})
		return
	}
	c.JSON(http.StatusOK, agents)
}

// SetStatus enables or disables an AI agent.
// PATCH /api/v1/agents/:id/status
// Body: { "active": true|false }
func (h *AgentsHandler) SetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var body struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	agent, err := h.userRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	if agent.AuthSource != "ai" {
		c.JSON(http.StatusForbidden, gin.H{"error": "not an AI agent"})
		return
	}

	if err := h.userRepo.SetActive(id, body.Active); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update agent status"})
		return
	}

	agent.Active = body.Active
	c.JSON(http.StatusOK, agent)
}

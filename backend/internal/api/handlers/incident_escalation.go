package handlers

import (
	"net/http"

	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EscalateIncident handles POST /api/v1/incidents/:id/escalate.
// Body: {"escalation_policy_id": "uuid"}
//
// Manually triggers an escalation for an incident, independent of the alert
// processing pipeline. Idempotent — triggering again while one is active is a no-op.
func EscalateIncident(engine services.EscalationEngine) gin.HandlerFunc {
	return func(c *gin.Context) {
		incidentID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid incident ID"})
			return
		}
		var req struct {
			EscalationPolicyID string `json:"escalation_policy_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		policyID, err := uuid.Parse(req.EscalationPolicyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid escalation_policy_id"})
			return
		}
		if err := engine.TriggerIncidentEscalation(incidentID, policyID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"ok": true, "incident_id": incidentID, "policy_id": policyID})
	}
}

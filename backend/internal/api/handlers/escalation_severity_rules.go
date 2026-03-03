package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/repository"
)

// ListSeverityRules handles GET /api/v1/escalation-policies/severity-rules
func ListSeverityRules(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		rules, err := repo.ListSeverityRules()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rules})
	}
}

// UpsertSeverityRule handles PUT /api/v1/escalation-policies/severity-rules/:severity
func UpsertSeverityRule(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		severity := c.Param("severity")
		var req struct {
			EscalationPolicyID string `json:"escalation_policy_id" binding:"required,uuid"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		policyID, _ := uuid.Parse(req.EscalationPolicyID)
		rule, err := repo.UpsertSeverityRule(severity, policyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, rule)
	}
}

// DeleteSeverityRule handles DELETE /api/v1/escalation-policies/severity-rules/:severity
func DeleteSeverityRule(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := repo.DeleteSeverityRule(c.Param("severity")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

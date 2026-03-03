package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/repository"
)

// GetEscalationSettings handles GET /api/v1/settings/escalation
func GetEscalationSettings(repo repository.SystemSettingsRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := repo.GetGlobalFallbackPolicyID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"global_fallback_policy_id": id})
	}
}

// UpdateEscalationSettings handles PUT /api/v1/settings/escalation
func UpdateEscalationSettings(repo repository.SystemSettingsRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			GlobalFallbackPolicyID *string `json:"global_fallback_policy_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var id *uuid.UUID
		if req.GlobalFallbackPolicyID != nil && *req.GlobalFallbackPolicyID != "" {
			parsed, err := uuid.Parse(*req.GlobalFallbackPolicyID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid UUID"})
				return
			}
			id = &parsed
		}
		if err := repo.SetGlobalFallbackPolicyID(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"global_fallback_policy_id": id})
	}
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/repository"
	"github.com/fluidify/regen/internal/services"
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

// GetSystemSettings handles GET /api/v1/settings/system
func GetSystemSettings(repo repository.SystemSettingsRepository, aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		name, _ := repo.GetString(repository.KeyInstanceName)
		tz, _ := repo.GetString(repository.KeyTimezone)
		aiKeyRaw, _ := repo.GetString(repository.KeyOpenAIAPIKey)

		aiKeyConfigured := aiSvc.IsEnabled() || aiKeyRaw != ""
		aiKeyLast4 := ""
		if aiKeyRaw != "" && len(aiKeyRaw) >= 4 {
			aiKeyLast4 = "..." + aiKeyRaw[len(aiKeyRaw)-4:]
		}

		c.JSON(http.StatusOK, gin.H{
			"instance_name":     name,
			"timezone":          tz,
			"ai_key_configured": aiKeyConfigured,
			"ai_key_last4":      aiKeyLast4,
		})
	}
}

// UpdateSystemSettings handles PATCH /api/v1/settings/system
func UpdateSystemSettings(repo repository.SystemSettingsRepository, aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			InstanceName  *string `json:"instance_name"`
			Timezone      *string `json:"timezone"`
			OpenAIAPIKey  *string `json:"openai_api_key"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.InstanceName != nil {
			if err := repo.SetString(repository.KeyInstanceName, *req.InstanceName); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save instance name"}); return
			}
		}
		if req.Timezone != nil {
			if err := repo.SetString(repository.KeyTimezone, *req.Timezone); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save timezone"}); return
			}
		}
		if req.OpenAIAPIKey != nil {
			if err := repo.SetString(repository.KeyOpenAIAPIKey, *req.OpenAIAPIKey); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save API key"}); return
			}
			aiSvc.Reload(*req.OpenAIAPIKey)
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// TestOpenAIKey handles POST /api/v1/settings/system/ai/test
// Accepts an optional key in the body; falls back to the currently configured key.
func TestOpenAIKey(repo repository.SystemSettingsRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			APIKey string `json:"openai_api_key"`
		}
		_ = c.ShouldBindJSON(&req)

		key := req.APIKey
		if key == "" {
			key, _ = repo.GetString(repository.KeyOpenAIAPIKey)
		}
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no API key provided or configured"})
			return
		}

		// Make a minimal models-list call to validate the key
		testSvc := services.NewAIService(key, "gpt-4o-mini", 10, 10)
		if !testSvc.IsEnabled() {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid key"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

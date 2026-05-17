package handlers

import (
	"net/http"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
// telemetryDisabled reflects the REGEN_NO_TELEMETRY env var so the UI can show
// whether telemetry was disabled at the infrastructure level (not just via the toggle).
func GetSystemSettings(repo repository.SystemSettingsRepository, aiSvc services.AIService, telemetryDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		name, _ := repo.GetString(repository.KeyInstanceName)
		tz, _ := repo.GetString(repository.KeyTimezone)
		aiKeyRaw, _ := repo.GetString(repository.KeyOpenAIAPIKey)
		aiProvider, _ := repo.GetString(repository.KeyAIProvider)
		if aiProvider == "" {
			aiProvider = "openai"
		}
		ollamaURL, _ := repo.GetString(repository.KeyOllamaBaseURL)
		ollamaModel, _ := repo.GetString(repository.KeyOllamaModel)

		aiKeyConfigured := aiSvc.IsEnabled() || aiKeyRaw != ""
		aiKeyLast4 := ""
		if aiKeyRaw != "" && len(aiKeyRaw) >= 4 {
			aiKeyLast4 = "..." + aiKeyRaw[len(aiKeyRaw)-4:]
		}

		optOut, _ := repo.GetTelemetryOptOut()
		telemetryEnabled := !telemetryDisabled && !optOut

		c.JSON(http.StatusOK, gin.H{
			"instance_name":      name,
			"timezone":           tz,
			"ai_provider":        aiProvider,
			"ai_key_configured":  aiKeyConfigured,
			"ai_key_last4":       aiKeyLast4,
			"ollama_base_url":    ollamaURL,
			"ollama_model":       ollamaModel,
			"telemetry_enabled":  telemetryEnabled,
			"telemetry_env_lock": telemetryDisabled,
		})
	}
}

// PatchTelemetrySettings handles PATCH /api/v1/settings/system/telemetry
func PatchTelemetrySettings(repo repository.SystemSettingsRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			TelemetryEnabled bool `json:"telemetry_enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := repo.SetTelemetryOptOut(!req.TelemetryEnabled); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save telemetry preference"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// UpdateSystemSettings handles PATCH /api/v1/settings/system
func UpdateSystemSettings(repo repository.SystemSettingsRepository, aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			InstanceName    *string `json:"instance_name"`
			Timezone        *string `json:"timezone"`
			OpenAIAPIKey    *string `json:"openai_api_key"`
			AIProvider      *string `json:"ai_provider"`
			AnthropicAPIKey *string `json:"anthropic_api_key"`
			OllamaBaseURL   *string `json:"ollama_base_url"`
			OllamaModel     *string `json:"ollama_model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.InstanceName != nil {
			if err := repo.SetString(repository.KeyInstanceName, *req.InstanceName); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save instance name"})
				return
			}
		}
		if req.Timezone != nil {
			if err := repo.SetString(repository.KeyTimezone, *req.Timezone); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save timezone"})
				return
			}
		}
		if req.OpenAIAPIKey != nil {
			if err := repo.SetString(repository.KeyOpenAIAPIKey, *req.OpenAIAPIKey); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save OpenAI API key"})
				return
			}
		}
		if req.AIProvider != nil {
			if err := repo.SetString(repository.KeyAIProvider, *req.AIProvider); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save AI provider"})
				return
			}
		}
		if req.AnthropicAPIKey != nil {
			if err := repo.SetString(repository.KeyAnthropicAPIKey, *req.AnthropicAPIKey); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save Anthropic API key"})
				return
			}
		}
		if req.OllamaBaseURL != nil {
			if err := repo.SetString(repository.KeyOllamaBaseURL, *req.OllamaBaseURL); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save Ollama base URL"})
				return
			}
		}
		if req.OllamaModel != nil {
			if err := repo.SetString(repository.KeyOllamaModel, *req.OllamaModel); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save Ollama model"})
				return
			}
		}
		// Reload AI service if any provider setting changed.
		if req.AIProvider != nil || req.OpenAIAPIKey != nil || req.AnthropicAPIKey != nil || req.OllamaBaseURL != nil || req.OllamaModel != nil {
			provider, _ := repo.GetString(repository.KeyAIProvider)
			if provider == "" {
				provider = "openai"
			}
			var apiKey, model, ollamaURL string
			switch provider {
			case "anthropic":
				apiKey, _ = repo.GetString(repository.KeyAnthropicAPIKey)
				model = "claude-haiku-4-5-20251001"
			case "ollama":
				ollamaURL, _ = repo.GetString(repository.KeyOllamaBaseURL)
				model, _ = repo.GetString(repository.KeyOllamaModel)
				if model == "" {
					model = "llama3"
				}
			default:
				apiKey, _ = repo.GetString(repository.KeyOpenAIAPIKey)
				model = "gpt-4o-mini"
			}
			aiSvc.Reload(provider, apiKey, model, ollamaURL)
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// TestAIKey handles POST /api/v1/settings/system/ai/test
// Validates that the given provider credentials are sufficient to enable AI features.
func TestAIKey(repo repository.SystemSettingsRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Provider    string `json:"provider"`
			APIKey      string `json:"api_key"`
			OllamaURL   string `json:"ollama_base_url"`
		}
		_ = c.ShouldBindJSON(&req)

		provider := req.Provider
		if provider == "" {
			provider, _ = repo.GetString(repository.KeyAIProvider)
		}
		if provider == "" {
			provider = "openai"
		}

		apiKey := req.APIKey
		ollamaURL := req.OllamaURL

		// Fall back to stored values
		switch provider {
		case "anthropic":
			if apiKey == "" {
				apiKey, _ = repo.GetString(repository.KeyAnthropicAPIKey)
			}
			if apiKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "no Anthropic API key provided or configured"})
				return
			}
		case "ollama":
			if ollamaURL == "" {
				ollamaURL, _ = repo.GetString(repository.KeyOllamaBaseURL)
			}
			if ollamaURL == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "no Ollama base URL provided or configured"})
				return
			}
		default:
			provider = "openai"
			if apiKey == "" {
				apiKey, _ = repo.GetString(repository.KeyOpenAIAPIKey)
			}
			if apiKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "no API key provided or configured"})
				return
			}
		}

		testSvc := services.NewAIService(provider, apiKey, "gpt-4o-mini", 10, 10, ollamaURL)
		if !testSvc.IsEnabled() {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

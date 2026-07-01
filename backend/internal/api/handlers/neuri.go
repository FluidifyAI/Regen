package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/api/handlers/dto"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ReceiveNeuriResult handles POST /api/v1/neuri/result.
// Unprotected — called by Neuri from inside the cluster (Alpha: no HMAC yet).
func ReceiveNeuriResult(
	incidentRepo repository.IncidentRepository,
	neuriRepo repository.NeuriResultRepository,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.NeuriResultRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		incidentID, err := uuid.Parse(req.IncidentID)
		if err != nil {
			dto.BadRequest(c, "invalid incident_id", map[string]interface{}{"incident_id": "must be a valid UUID"})
			return
		}

		investigationRunID, err := uuid.Parse(req.InvestigationRunID)
		if err != nil {
			dto.BadRequest(c, "invalid investigation_run_id", map[string]interface{}{"investigation_run_id": "must be a valid UUID"})
			return
		}

		if _, err := incidentRepo.GetByID(incidentID); err != nil {
			if _, ok := err.(*repository.NotFoundError); ok {
				dto.NotFound(c, "incident", req.IncidentID)
				return
			}
			dto.InternalError(c, err)
			return
		}

		ranked := req.RankedHypotheses
		if ranked == nil {
			ranked = json.RawMessage("[]")
		}

		result := &models.NeuriResult{
			IncidentID:         incidentID,
			InvestigationRunID: investigationRunID,
			TopHypothesis:      req.TopHypothesis,
			Confidence:         req.Confidence,
			Summary:            req.Summary,
			RankedHypotheses:   models.RawJSON(ranked),
		}

		if err := neuriRepo.Create(result); err != nil {
			slog.Error("failed to store neuri result",
				"error", err,
				"incident_id", req.IncidentID,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.NeuriResultResponse{
			ID:         result.ID.String(),
			IncidentID: result.IncidentID.String(),
		})
	}
}

// ListNeuriResults handles GET /api/v1/neuri/result?incident_id=<uuid>.
// Protected — called by the frontend to poll for investigation results.
func ListNeuriResults(neuriRepo repository.NeuriResultRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Query("incident_id")
		if raw == "" {
			dto.BadRequest(c, "incident_id query parameter is required", nil)
			return
		}
		incidentID, err := uuid.Parse(raw)
		if err != nil {
			dto.BadRequest(c, "invalid incident_id", map[string]interface{}{"incident_id": "must be a valid UUID"})
			return
		}

		results, err := neuriRepo.ListByIncidentID(incidentID)
		if err != nil {
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"results": results})
	}
}

// TriggerNeuriInvestigation handles POST /api/v1/neuri/investigate.
// Protected — proxies to the Neuri webhook using credentials from system_settings,
// so the secret never touches the browser.
func TriggerNeuriInvestigation(
	incidentRepo repository.IncidentRepository,
	settingsRepo repository.SystemSettingsRepository,
) gin.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}
	return func(c *gin.Context) {
		var req dto.NeuriTriggerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		incidentID, err := uuid.Parse(req.IncidentID)
		if err != nil {
			dto.BadRequest(c, "invalid incident_id", map[string]interface{}{"incident_id": "must be a valid UUID"})
			return
		}

		incident, err := incidentRepo.GetByID(incidentID)
		if err != nil {
			if _, ok := err.(*repository.NotFoundError); ok {
				dto.NotFound(c, "incident", req.IncidentID)
				return
			}
			dto.InternalError(c, err)
			return
		}

		webhookURL, _ := settingsRepo.GetString(repository.KeyNeuriWebhookURL)
		secret, _ := settingsRepo.GetString(repository.KeyNeuriWebhookSecret)
		regenBaseURL, _ := settingsRepo.GetString(repository.KeyNeuriRegenBaseURL)

		if webhookURL == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"code":    "neuri_not_configured",
					"message": "Neuri is not configured. Set the webhook URL in Settings → System → Neuri.",
				},
			})
			return
		}

		callbackURL := regenBaseURL + "/api/v1/neuri/result"

		payload := map[string]interface{}{
			"incident_id":   incident.ID.String(),
			"title":         incident.Title,
			"started_at":    incident.TriggeredAt.UTC().Format(time.RFC3339),
			"severity":      incident.Severity,
			"affected_services": []string{},
			"callback_url":  callbackURL,
		}
		body, _ := json.Marshal(payload)

		outReq, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			webhookURL,
			bytes.NewReader(body),
		)
		if err != nil {
			dto.InternalError(c, fmt.Errorf("building neuri request: %w", err))
			return
		}
		outReq.Header.Set("Content-Type", "application/json")
		if secret != "" {
			outReq.Header.Set("Authorization", "Bearer "+secret)
		}

		resp, err := client.Do(outReq)
		if err != nil {
			slog.Error("neuri webhook call failed",
				"error", err,
				"incident_id", incident.ID,
				"request_id", c.GetString("request_id"),
			)
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{
					"code":    "neuri_unreachable",
					"message": "Could not reach Neuri. Check the webhook URL in Settings → System → Neuri.",
				},
			})
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body) //nolint:errcheck

		if resp.StatusCode >= 500 {
			slog.Error("neuri webhook returned server error",
				"status", resp.StatusCode,
				"incident_id", incident.ID,
				"request_id", c.GetString("request_id"),
			)
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{"code": "neuri_error", "message": "Neuri returned an error. Investigation may not have started."},
			})
			return
		}

		c.JSON(http.StatusAccepted, dto.NeuriTriggerResponse{Status: "accepted"})
	}
}

// GetNeuriSettings handles GET /api/v1/settings/neuri.
// Returns the configured URL and base URL; the secret is masked (last 4 chars only).
func GetNeuriSettings(settingsRepo repository.SystemSettingsRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		webhookURL, _ := settingsRepo.GetString(repository.KeyNeuriWebhookURL)
		secret, _ := settingsRepo.GetString(repository.KeyNeuriWebhookSecret)
		regenBaseURL, _ := settingsRepo.GetString(repository.KeyNeuriRegenBaseURL)

		resp := dto.NeuriSettingsResponse{
			WebhookURL:       webhookURL,
			RegenBaseURL:     regenBaseURL,
			WebhookSecretSet: secret != "",
		}
		if len(secret) >= 4 {
			resp.WebhookSecretHint = "••••" + secret[len(secret)-4:]
		} else if secret != "" {
			resp.WebhookSecretHint = "••••"
		}

		c.JSON(http.StatusOK, resp)
	}
}

// UpdateNeuriSettings handles PATCH /api/v1/settings/neuri.
// Empty string fields are skipped (preserves existing value).
func UpdateNeuriSettings(settingsRepo repository.SystemSettingsRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.NeuriSettingsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		if req.WebhookURL != "" {
			if err := settingsRepo.SetString(repository.KeyNeuriWebhookURL, req.WebhookURL); err != nil {
				dto.InternalError(c, err)
				return
			}
		}
		if req.RegenBaseURL != "" {
			if err := settingsRepo.SetString(repository.KeyNeuriRegenBaseURL, req.RegenBaseURL); err != nil {
				dto.InternalError(c, err)
				return
			}
		}
		if req.WebhookSecret != "" {
			if err := settingsRepo.SetString(repository.KeyNeuriWebhookSecret, req.WebhookSecret); err != nil {
				dto.InternalError(c, err)
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

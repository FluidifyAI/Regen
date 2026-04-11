package handlers

import (
	"log/slog"
	"net/http"

	"github.com/FluidifyAI/Regen/backend/internal/api/handlers/dto"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListAlerts handles GET /api/v1/alerts.
//
// Returns a paginated list of alerts, optionally filtered by source, status, and severity.
func ListAlerts(alertRepo repository.AlertRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var filters dto.AlertFilters
		if err := c.ShouldBindQuery(&filters); err != nil {
			dto.ValidationError(c, err)
			return
		}

		var pagination dto.PaginationParams
		if err := c.ShouldBindQuery(&pagination); err != nil {
			dto.ValidationError(c, err)
			return
		}
		pagination.Normalize()

		alerts, total, err := alertRepo.List(filters.ToRepository(), pagination.ToRepository())
		if err != nil {
			slog.Error("failed to list alerts",
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		responses := make([]dto.AlertResponse, len(alerts))
		for i := range alerts {
			responses[i] = dto.ToAlertResponse(&alerts[i])
		}

		c.JSON(http.StatusOK, dto.PaginatedResponse{
			Data:   responses,
			Total:  total,
			Limit:  pagination.PageSize,
			Offset: (pagination.Page - 1) * pagination.PageSize,
		})
	}
}

// GetAlert handles GET /api/v1/alerts/:id.
func GetAlert(alertRepo repository.AlertRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid alert ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}

		alert, err := alertRepo.GetByID(id)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "alert", id.String())
				return
			}
			slog.Error("failed to get alert",
				"alert_id", id,
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.ToAlertResponse(alert))
	}
}

// acknowledgeAlertRequest is the request body for POST /api/v1/alerts/:id/acknowledge
type acknowledgeAlertRequest struct {
	UserName        string `json:"user_name"        binding:"required"`
	AcknowledgedVia string `json:"acknowledged_via"`
}

// AcknowledgeAlert handles POST /api/v1/alerts/:id/acknowledge.
//
// Marks the alert as acknowledged (stopping further escalation) and appends a
// timeline entry to the linked incident (if any). Idempotent.
func AcknowledgeAlert(
	alertRepo repository.AlertRepository,
	escalationEngine services.EscalationEngine,
	incidentRepo repository.IncidentRepository,
	timelineRepo repository.TimelineRepository,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid alert ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}

		var req acknowledgeAlertRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Confirm alert exists before touching escalation state
		if _, err := alertRepo.GetByID(id); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "alert", id.String())
				return
			}
			slog.Error("failed to fetch alert for acknowledgment",
				"alert_id", id,
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		via := models.AcknowledgmentVia(req.AcknowledgedVia)
		if via == "" {
			via = models.AcknowledgmentViaAPI
		}

		if err := services.AcknowledgeAlertWithTimeline(id, req.UserName, via, escalationEngine, incidentRepo, timelineRepo); err != nil {
			slog.Error("failed to acknowledge alert",
				"alert_id", id,
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		// Return 204 No Content — the acknowledgment timestamp is written by the
		// repository inside a transaction; fabricating time.Now() here would drift
		// from the persisted value and break audit correctness.
		// AbortWithStatus flushes the header immediately (needed in test contexts).
		c.AbortWithStatus(http.StatusNoContent)
	}
}

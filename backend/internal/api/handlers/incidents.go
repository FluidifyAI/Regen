package handlers

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
)

// ListIncidents handles GET /api/v1/incidents
func ListIncidents(incidentSvc services.IncidentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse query parameters
		var filters dto.IncidentFilters
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

		// Fetch incidents from service
		incidents, total, err := incidentSvc.ListIncidents(
			filters.ToRepository(),
			pagination.ToRepository(),
		)
		if err != nil {
			slog.Error("failed to list incidents",
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		// Convert to response DTOs
		responses := make([]dto.IncidentResponse, len(incidents))
		for i, incident := range incidents {
			responses[i] = dto.ToIncidentResponse(&incident)
		}

		// Return paginated response
		c.JSON(200, dto.PaginatedResponse{
			Data:   responses,
			Total:  total,
			Limit:  pagination.PageSize,
			Offset: (pagination.Page - 1) * pagination.PageSize,
		})
	}
}

// GetIncident handles GET /api/v1/incidents/:id
func GetIncident(incidentSvc services.IncidentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")

		// Parse identifier (UUID or incident number)
		uid, num, err := parseIncidentIdentifier(idParam)
		if err != nil {
			dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
				"id": "must be a valid UUID or incident number",
			})
			return
		}

		// Fetch incident
		incident, err := incidentSvc.GetIncident(uid, num)
		if err != nil {
			if _, ok := err.(*repository.NotFoundError); ok {
				dto.NotFound(c, "incident", idParam)
				return
			}
			dto.InternalError(c, err)
			return
		}

		// Fetch linked alerts
		alerts, err := incidentSvc.GetIncidentAlerts(incident.ID)
		if err != nil {
			slog.Error("failed to fetch incident alerts",
				"error", err,
				"incident_id", incident.ID,
			)
			alerts = []models.Alert{} // Continue with empty alerts
		}

		// Fetch recent timeline (last 50 by default)
		timeline, _, err := incidentSvc.GetIncidentTimeline(incident.ID, repository.Pagination{
			Page:     1,
			PageSize: 50,
		})
		if err != nil {
			slog.Error("failed to fetch incident timeline",
				"error", err,
				"incident_id", incident.ID,
			)
			timeline = []models.TimelineEntry{} // Continue with empty timeline
		}

		// Build response
		resp := dto.IncidentDetailResponse{
			IncidentResponse: dto.ToIncidentResponse(incident),
			Alerts:           make([]dto.AlertSummary, len(alerts)),
			Timeline:         make([]dto.TimelineEntrySummary, len(timeline)),
		}

		for i, alert := range alerts {
			resp.Alerts[i] = dto.AlertSummary{
				ID:         alert.ID,
				Title:      alert.Title,
				Severity:   string(alert.Severity),
				Status:     string(alert.Status),
				ReceivedAt: alert.ReceivedAt,
			}
		}

		for i, entry := range timeline {
			resp.Timeline[i] = dto.TimelineEntrySummary{
				ID:        entry.ID,
				Timestamp: entry.Timestamp,
				Type:      entry.Type,
				ActorType: entry.ActorType,
				ActorID:   entry.ActorID,
				Content:   entry.Content,
			}
		}

		c.JSON(200, resp)
	}
}

// CreateIncident handles POST /api/v1/incidents
func CreateIncident(incidentSvc services.IncidentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request body
		var req dto.CreateIncidentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Create incident via service
		params := services.CreateIncidentParams{
			Title:       req.Title,
			Severity:    models.IncidentSeverity(req.Severity),
			Description: req.Description,
			CreatedBy:   "user", // For v0.1, hardcoded. Will use auth context in v0.2+
		}

		// Default severity to medium if not specified
		if params.Severity == "" {
			params.Severity = models.IncidentSeverityMedium
		}

		incident, err := incidentSvc.CreateIncident(&params)
		if err != nil {
			slog.Error("failed to create incident",
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		// Return created incident
		c.JSON(201, dto.ToIncidentResponse(incident))
	}
}

// UpdateIncident handles PATCH /api/v1/incidents/:id
func UpdateIncident(incidentSvc services.IncidentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")

		// Parse identifier
		uid, num, err := parseIncidentIdentifier(idParam)
		if err != nil {
			dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
				"id": "must be a valid UUID or incident number",
			})
			return
		}

		// Parse request body
		var req dto.UpdateIncidentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Fetch current incident
		incident, err := incidentSvc.GetIncident(uid, num)
		if err != nil {
			if _, ok := err.(*repository.NotFoundError); ok {
				dto.NotFound(c, "incident", idParam)
				return
			}
			dto.InternalError(c, err)
			return
		}

		// Validate status transition if status is being changed
		if req.Status != "" && req.Status != string(incident.Status) {
			requestedStatus := models.IncidentStatus(req.Status)
			if !isValidTransition(incident.Status, requestedStatus) {
				dto.Conflict(c, "Invalid status transition", map[string]interface{}{
					"current_status":    string(incident.Status),
					"requested_status":  req.Status,
					"valid_transitions": getValidTransitions(incident.Status),
				})
				return
			}
		}

		// Update incident
		params := services.UpdateIncidentParams{
			Status:    models.IncidentStatus(req.Status),
			Severity:  models.IncidentSeverity(req.Severity),
			Summary:   req.Summary,
			UpdatedBy: "user",        // For v0.1, hardcoded
			ClientIP:  c.ClientIP(), // For audit logging
		}

		updatedIncident, err := incidentSvc.UpdateIncident(incident.ID, &params)
		if err != nil {
			dto.InternalError(c, err)
			return
		}

		c.JSON(200, dto.ToIncidentResponse(updatedIncident))
	}
}

// parseIncidentIdentifier parses an incident identifier (UUID or number)
func parseIncidentIdentifier(id string) (uuid.UUID, int, error) {
	// Try parsing as incident number first
	if num, err := strconv.Atoi(id); err == nil {
		return uuid.Nil, num, nil
	}

	// Try parsing as UUID
	if uid, err := uuid.Parse(id); err == nil {
		return uid, 0, nil
	}

	return uuid.Nil, 0, fmt.Errorf("invalid incident identifier")
}

// State machine validation
var validTransitions = map[models.IncidentStatus][]models.IncidentStatus{
	models.IncidentStatusTriggered: {
		models.IncidentStatusAcknowledged,
		models.IncidentStatusResolved,
		models.IncidentStatusCanceled,
	},
	models.IncidentStatusAcknowledged: {
		models.IncidentStatusResolved,
	},
	models.IncidentStatusResolved: {},
	models.IncidentStatusCanceled: {},
}

func isValidTransition(current, requested models.IncidentStatus) bool {
	if current == requested {
		return true // No-op
	}

	allowed := validTransitions[current]
	for _, status := range allowed {
		if status == requested {
			return true
		}
	}
	return false
}

func getValidTransitions(current models.IncidentStatus) []string {
	allowed := validTransitions[current]
	transitions := make([]string, len(allowed))
	for i, status := range allowed {
		transitions[i] = string(status)
	}
	return transitions
}

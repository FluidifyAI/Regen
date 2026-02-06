package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
)

// GetIncidentTimeline handles GET /api/v1/incidents/:id/timeline
func GetIncidentTimeline(incidentSvc services.IncidentService) gin.HandlerFunc {
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

		// Fetch incident to verify existence
		incident, err := incidentSvc.GetIncident(uid, num)
		if err != nil {
			if _, ok := err.(*repository.NotFoundError); ok {
				dto.NotFound(c, "incident", idParam)
				return
			}
			dto.InternalError(c, err)
			return
		}

		// Parse pagination
		var pagination dto.PaginationParams
		if err := c.ShouldBindQuery(&pagination); err != nil {
			dto.ValidationError(c, err)
			return
		}
		pagination.Normalize()

		// Fetch timeline entries
		entries, total, err := incidentSvc.GetIncidentTimeline(incident.ID, pagination.ToRepository())
		if err != nil {
			dto.InternalError(c, err)
			return
		}

		// Convert to response DTOs
		responses := make([]dto.TimelineEntryResponse, len(entries))
		for i, entry := range entries {
			responses[i] = dto.ToTimelineEntryResponse(&entry)
		}

		c.JSON(200, dto.PaginatedResponse{
			Data:   responses,
			Total:  total,
			Limit:  pagination.PageSize,
			Offset: (pagination.Page - 1) * pagination.PageSize,
		})
	}
}

// CreateTimelineEntry handles POST /api/v1/incidents/:id/timeline
func CreateTimelineEntry(incidentSvc services.IncidentService) gin.HandlerFunc {
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

		// Parse request
		var req dto.CreateTimelineEntryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Validate type (only "message" allowed for manual entries)
		if req.Type != "message" {
			dto.BadRequest(c, "Invalid timeline entry type", map[string]interface{}{
				"type": "manual entries must have type='message'",
			})
			return
		}

		// Create entry
		params := services.CreateTimelineEntryParams{
			IncidentID: incident.ID,
			Type:       req.Type,
			Content:    req.Content,
			ActorType:  "user",
			ActorID:    "anonymous", // For v0.1. Will use auth context in v0.2+
		}

		entry, err := incidentSvc.CreateTimelineEntry(&params)
		if err != nil {
			dto.InternalError(c, err)
			return
		}

		c.JSON(201, dto.ToTimelineEntryResponse(entry))
	}
}

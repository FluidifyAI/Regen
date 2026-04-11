package handlers

import (
	"github.com/FluidifyAI/Regen/backend/internal/api/handlers/dto"
	"github.com/FluidifyAI/Regen/backend/internal/api/middleware"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// resolveActorNames takes a slice of timeline responses and fills in ActorName
// for entries where actor_type == "user" and actor_id is a valid UUID.
// A single map lookup per unique user avoids N+1 queries.
func resolveActorNames(entries []dto.TimelineEntryResponse, userRepo repository.UserRepository) {
	if userRepo == nil {
		return
	}
	// Collect unique user UUIDs
	seen := map[uuid.UUID]string{} // uuid → resolved name (empty = pending)
	for _, e := range entries {
		if e.ActorType == "user" && e.ActorID != "" {
			if uid, err := uuid.Parse(e.ActorID); err == nil {
				seen[uid] = ""
			}
		}
	}
	// Fetch each unique user once
	for uid := range seen {
		if user, err := userRepo.GetByID(uid); err == nil {
			seen[uid] = user.Name
		}
	}
	// Apply resolved names back to entries
	for i, e := range entries {
		if e.ActorType == "user" && e.ActorID != "" {
			if uid, err := uuid.Parse(e.ActorID); err == nil {
				if name := seen[uid]; name != "" {
					entries[i].ActorName = name
				}
			}
		}
	}
}

// GetIncidentTimeline handles GET /api/v1/incidents/:id/timeline
func GetIncidentTimeline(incidentSvc services.IncidentService, userRepo repository.UserRepository) gin.HandlerFunc {
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
		resolveActorNames(responses, userRepo)

		c.JSON(200, dto.PaginatedResponse{
			Data:   responses,
			Total:  total,
			Limit:  pagination.PageSize,
			Offset: (pagination.Page - 1) * pagination.PageSize,
		})
	}
}

// CreateTimelineEntry handles POST /api/v1/incidents/:id/timeline
func CreateTimelineEntry(incidentSvc services.IncidentService, userRepo repository.UserRepository) gin.HandlerFunc {
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

		// Resolve the logged-in user so timeline entries show the real name.
		actorID := "anonymous"
		if user := middleware.GetLocalUser(c); user != nil {
			actorID = user.ID.String()
		}

		// Create entry
		params := services.CreateTimelineEntryParams{
			IncidentID: incident.ID,
			Type:       req.Type,
			Content:    req.Content,
			ActorType:  "user",
			ActorID:    actorID,
		}

		entry, err := incidentSvc.CreateTimelineEntry(&params)
		if err != nil {
			dto.InternalError(c, err)
			return
		}

		resp := dto.ToTimelineEntryResponse(entry)
		// Resolve the actor name for the newly created entry
		single := []dto.TimelineEntryResponse{resp}
		resolveActorNames(single, userRepo)
		c.JSON(201, single[0])
	}
}

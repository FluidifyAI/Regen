package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
)

// isNotFound checks if an error is a repository NotFoundError.
// Use this instead of checking gorm.ErrRecordNotFound directly — repositories
// wrap that sentinel in *repository.NotFoundError before returning it.
func isNotFound(err error) bool {
	_, ok := err.(*repository.NotFoundError)
	return ok
}

// isBuiltInConflict checks if an error is a BuiltInTemplateError (attempt to delete a built-in).
func isBuiltInConflict(err error) bool {
	_, ok := err.(*services.BuiltInTemplateError)
	return ok
}

// resolveIncident parses the :id URL param and fetches the incident.
// Returns (incident, true) on success or writes the error response and returns (nil, false).
func resolveIncident(c *gin.Context, svc services.IncidentService) (*models.Incident, bool) {
	idParam := c.Param("id")
	uid, num, err := parseIncidentIdentifier(idParam)
	if err != nil {
		dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
			"id": "must be a valid UUID or incident number",
		})
		return nil, false
	}
	incident, err := svc.GetIncident(uid, num)
	if err != nil {
		if isNotFound(err) {
			dto.NotFound(c, "incident", idParam)
			return nil, false
		}
		dto.InternalError(c, err)
		return nil, false
	}
	return incident, true
}

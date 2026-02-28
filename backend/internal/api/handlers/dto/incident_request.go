package dto

import (
	"time"

	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
)

// CreateIncidentRequest is the request body for POST /api/v1/incidents
type CreateIncidentRequest struct {
	Title       string `json:"title" binding:"required,min=1,max=500"`
	Severity    string `json:"severity" binding:"omitempty,oneof=critical high medium low"`
	Description string `json:"description" binding:"max=10000"` // Max 10K chars for description
	// AIEnabled controls whether AI agents process this incident. Defaults to true.
	AIEnabled *bool `json:"ai_enabled"`
}

// UpdateIncidentRequest is the request body for PATCH /api/v1/incidents/:id
type UpdateIncidentRequest struct {
	Status   string `json:"status" binding:"omitempty,oneof=triggered acknowledged resolved canceled"`
	Severity string `json:"severity" binding:"omitempty,oneof=critical high medium low"`
	Summary  string `json:"summary" binding:"max=5000"` // Max 5K chars for summary
	// AIEnabled can toggle AI agent processing on/off after creation.
	AIEnabled *bool `json:"ai_enabled"`
}

// IncidentFilters holds query parameters for filtering incidents
type IncidentFilters struct {
	Status        string     `form:"status" binding:"omitempty,oneof=triggered acknowledged resolved canceled"`
	Severity      string     `form:"severity" binding:"omitempty,oneof=critical high medium low"`
	CreatedAfter  *time.Time `form:"created_after" time_format:"2006-01-02T15:04:05Z07:00"`
	CreatedBefore *time.Time `form:"created_before" time_format:"2006-01-02T15:04:05Z07:00"`
}

// ToRepository converts API filters to repository filters
func (f *IncidentFilters) ToRepository() repository.IncidentFilters {
	return repository.IncidentFilters{
		Status:    models.IncidentStatus(f.Status),
		Severity:  models.IncidentSeverity(f.Severity),
		StartDate: f.CreatedAfter,
		EndDate:   f.CreatedBefore,
	}
}

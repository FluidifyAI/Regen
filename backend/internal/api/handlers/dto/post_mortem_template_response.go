package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
)

// PostMortemTemplateResponse is the API representation of a post-mortem template.
type PostMortemTemplateResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Sections    []string  `json:"sections"`
	IsBuiltIn   bool      `json:"is_built_in"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func ToPostMortemTemplateResponse(t *models.PostMortemTemplate) PostMortemTemplateResponse {
	sections := []string(t.Sections)
	if sections == nil {
		sections = []string{}
	}
	return PostMortemTemplateResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Sections:    sections,
		IsBuiltIn:   t.IsBuiltIn,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

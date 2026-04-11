package dto

import (
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

// ActionItemResponse is the API representation of an action item.
type ActionItemResponse struct {
	ID           uuid.UUID  `json:"id"`
	PostMortemID uuid.UUID  `json:"post_mortem_id"`
	Title        string     `json:"title"`
	Owner        *string    `json:"owner,omitempty"`
	DueDate      *time.Time `json:"due_date,omitempty"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func ToActionItemResponse(a *models.ActionItem) ActionItemResponse {
	return ActionItemResponse{
		ID:           a.ID,
		PostMortemID: a.PostMortemID,
		Title:        a.Title,
		Owner:        a.Owner,
		DueDate:      a.DueDate,
		Status:       string(a.Status),
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// PostMortemResponse is the full API representation of a post-mortem document.
type PostMortemResponse struct {
	ID           uuid.UUID            `json:"id"`
	IncidentID   uuid.UUID            `json:"incident_id"`
	TemplateID   *uuid.UUID           `json:"template_id,omitempty"`
	TemplateName string               `json:"template_name"`
	Status       string               `json:"status"`
	Content      string               `json:"content"`
	GeneratedBy  string               `json:"generated_by"`
	GeneratedAt  *time.Time           `json:"generated_at,omitempty"`
	PublishedAt  *time.Time           `json:"published_at,omitempty"`
	CreatedByID  string               `json:"created_by_id"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
	ActionItems  []ActionItemResponse `json:"action_items"`
}

// PostMortemCommentResponse is the API representation of a post-mortem comment.
type PostMortemCommentResponse struct {
	ID           uuid.UUID `json:"id"`
	PostMortemID uuid.UUID `json:"post_mortem_id"`
	AuthorID     string    `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
}

func ToPostMortemCommentResponse(c *models.PostMortemComment) PostMortemCommentResponse {
	return PostMortemCommentResponse{
		ID:           c.ID,
		PostMortemID: c.PostMortemID,
		AuthorID:     c.AuthorID,
		AuthorName:   c.AuthorName,
		Content:      c.Content,
		CreatedAt:    c.CreatedAt,
	}
}

// ListCommentsResponse wraps the comment list.
type ListCommentsResponse struct {
	Data []PostMortemCommentResponse `json:"data"`
}

func ToPostMortemResponse(pm *models.PostMortem) PostMortemResponse {
	items := make([]ActionItemResponse, 0, len(pm.ActionItems))
	for i := range pm.ActionItems {
		items = append(items, ToActionItemResponse(&pm.ActionItems[i]))
	}
	return PostMortemResponse{
		ID:           pm.ID,
		IncidentID:   pm.IncidentID,
		TemplateID:   pm.TemplateID,
		TemplateName: pm.TemplateName,
		Status:       string(pm.Status),
		Content:      pm.Content,
		GeneratedBy:  pm.GeneratedBy,
		GeneratedAt:  pm.GeneratedAt,
		PublishedAt:  pm.PublishedAt,
		CreatedByID:  pm.CreatedByID,
		CreatedAt:    pm.CreatedAt,
		UpdatedAt:    pm.UpdatedAt,
		ActionItems:  items,
	}
}

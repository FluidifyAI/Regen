package dto

import (
	"time"

	"github.com/google/uuid"
)

// GeneratePostMortemRequest is the body for POST /api/v1/incidents/:id/postmortem/generate.
type GeneratePostMortemRequest struct {
	TemplateID *uuid.UUID `json:"template_id"` // optional; uses first built-in if omitted
}

// UpdatePostMortemRequest is the body for PATCH /api/v1/incidents/:id/postmortem.
type UpdatePostMortemRequest struct {
	Content *string `json:"content" binding:"omitempty"`
	Status  *string `json:"status"  binding:"omitempty,oneof=draft published"`
}

// CreateActionItemRequest is the body for POST /api/v1/incidents/:id/postmortem/action-items.
type CreateActionItemRequest struct {
	Title   string     `json:"title"    binding:"required,min=1,max=500"`
	Owner   *string    `json:"owner"    binding:"omitempty,max=255"`
	DueDate *time.Time `json:"due_date" binding:"omitempty"`
}

// UpdateActionItemRequest is the body for PATCH .../action-items/:itemId.
type UpdateActionItemRequest struct {
	Title   *string    `json:"title"    binding:"omitempty,min=1,max=500"`
	Owner   *string    `json:"owner"    binding:"omitempty,max=255"`
	DueDate *time.Time `json:"due_date" binding:"omitempty"`
	Status  *string    `json:"status"   binding:"omitempty,oneof=open in_progress closed"`
}

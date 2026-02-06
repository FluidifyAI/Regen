package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
)

// TimelineEntryResponse is the response format for timeline entries
type TimelineEntryResponse struct {
	ID        uuid.UUID              `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	ActorType string                 `json:"actor_type"`
	ActorID   string                 `json:"actor_id,omitempty"`
	Content   map[string]interface{} `json:"content"`
}

// ToTimelineEntryResponse converts a models.TimelineEntry to TimelineEntryResponse
func ToTimelineEntryResponse(entry *models.TimelineEntry) TimelineEntryResponse {
	return TimelineEntryResponse{
		ID:        entry.ID,
		Timestamp: entry.Timestamp,
		Type:      entry.Type,
		ActorType: entry.ActorType,
		ActorID:   entry.ActorID,
		Content:   entry.Content,
	}
}

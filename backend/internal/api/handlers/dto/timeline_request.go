package dto

// CreateTimelineEntryRequest is the request body for POST /api/v1/incidents/:id/timeline
type CreateTimelineEntryRequest struct {
	Type    string                 `json:"type" binding:"required,eq=message"`
	Content map[string]interface{} `json:"content" binding:"required"`
}

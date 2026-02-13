package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/services"
)

// ScheduleResponse is the response body for schedule endpoints.
type ScheduleResponse struct {
	ID                  uuid.UUID       `json:"id"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	Timezone            string          `json:"timezone"`
	NotificationChannel string          `json:"notification_channel"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	Layers              []LayerResponse `json:"layers,omitempty"`
}

// LayerResponse is the response body for a schedule layer.
type LayerResponse struct {
	ID                   uuid.UUID             `json:"id"`
	ScheduleID           uuid.UUID             `json:"schedule_id"`
	Name                 string                `json:"name"`
	OrderIndex           int                   `json:"order_index"`
	RotationType         models.RotationType   `json:"rotation_type"`
	RotationStart        time.Time             `json:"rotation_start"`
	ShiftDurationSeconds int                   `json:"shift_duration_seconds"`
	CreatedAt            time.Time             `json:"created_at"`
	Participants         []ParticipantResponse `json:"participants,omitempty"`
}

// ParticipantResponse is the response body for a schedule participant.
type ParticipantResponse struct {
	ID         uuid.UUID `json:"id"`
	LayerID    uuid.UUID `json:"layer_id"`
	UserName   string    `json:"user_name"`
	OrderIndex int       `json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
}

// OverrideResponse is the response body for a schedule override.
type OverrideResponse struct {
	ID           uuid.UUID `json:"id"`
	ScheduleID   uuid.UUID `json:"schedule_id"`
	OverrideUser string    `json:"override_user"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// OnCallResponse is the response body for GET /schedules/:id/oncall.
type OnCallResponse struct {
	ScheduleID uuid.UUID `json:"schedule_id"`
	At         time.Time `json:"at"`
	UserName   string    `json:"user_name"` // empty string if nobody configured
	IsOverride bool      `json:"is_override"`
}

// TimelineResponse is the response body for GET /schedules/:id/oncall/timeline.
type TimelineResponse struct {
	ScheduleID uuid.UUID                  `json:"schedule_id"`
	From       time.Time                  `json:"from"`
	To         time.Time                  `json:"to"`
	Segments   []services.TimelineSegment `json:"segments"`
}

// ToScheduleResponse converts a models.Schedule to ScheduleResponse.
func ToScheduleResponse(s *models.Schedule) ScheduleResponse {
	resp := ScheduleResponse{
		ID:                  s.ID,
		Name:                s.Name,
		Description:         s.Description,
		Timezone:            s.Timezone,
		NotificationChannel: s.NotificationChannel,
		CreatedAt:           s.CreatedAt,
		UpdatedAt:           s.UpdatedAt,
	}
	if s.Layers != nil {
		resp.Layers = make([]LayerResponse, len(s.Layers))
		for i, l := range s.Layers {
			resp.Layers[i] = ToLayerResponse(&l)
		}
	}
	return resp
}

// ToLayerResponse converts a models.ScheduleLayer to LayerResponse.
func ToLayerResponse(l *models.ScheduleLayer) LayerResponse {
	resp := LayerResponse{
		ID:                   l.ID,
		ScheduleID:           l.ScheduleID,
		Name:                 l.Name,
		OrderIndex:           l.OrderIndex,
		RotationType:         l.RotationType,
		RotationStart:        l.RotationStart,
		ShiftDurationSeconds: l.ShiftDurationSeconds,
		CreatedAt:            l.CreatedAt,
	}
	if l.Participants != nil {
		resp.Participants = make([]ParticipantResponse, len(l.Participants))
		for i, p := range l.Participants {
			resp.Participants[i] = ParticipantResponse{
				ID:         p.ID,
				LayerID:    p.LayerID,
				UserName:   p.UserName,
				OrderIndex: p.OrderIndex,
				CreatedAt:  p.CreatedAt,
			}
		}
	}
	return resp
}

// ToOverrideResponse converts a models.ScheduleOverride to OverrideResponse.
func ToOverrideResponse(o *models.ScheduleOverride) OverrideResponse {
	return OverrideResponse{
		ID:           o.ID,
		ScheduleID:   o.ScheduleID,
		OverrideUser: o.OverrideUser,
		StartTime:    o.StartTime,
		EndTime:      o.EndTime,
		CreatedBy:    o.CreatedBy,
		CreatedAt:    o.CreatedAt,
	}
}

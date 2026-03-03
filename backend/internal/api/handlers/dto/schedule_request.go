package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
)

// CreateScheduleRequest is the request body for POST /api/v1/schedules.
type CreateScheduleRequest struct {
	Name                string `json:"name"                 binding:"required,min=1,max=255"`
	Description         string `json:"description"          binding:"max=1000"`
	Timezone            string `json:"timezone"             binding:"required,max=100"`
	NotificationChannel string `json:"notification_channel" binding:"max=255"`
}

// ToModel converts CreateScheduleRequest to models.Schedule.
func (r *CreateScheduleRequest) ToModel() *models.Schedule {
	return &models.Schedule{
		Name:                r.Name,
		Description:         r.Description,
		Timezone:            r.Timezone,
		NotificationChannel: r.NotificationChannel,
	}
}

// UpdateScheduleRequest is the request body for PATCH /api/v1/schedules/:id.
type UpdateScheduleRequest struct {
	Name                *string `json:"name"                 binding:"omitempty,min=1,max=255"`
	Description         *string `json:"description"          binding:"omitempty,max=1000"`
	Timezone            *string `json:"timezone"             binding:"omitempty,max=100"`
	NotificationChannel *string `json:"notification_channel" binding:"omitempty,max=255"`
}

// ApplyTo applies UpdateScheduleRequest fields to an existing Schedule model.
func (r *UpdateScheduleRequest) ApplyTo(s *models.Schedule) {
	if r.Name != nil {
		s.Name = *r.Name
	}
	if r.Description != nil {
		s.Description = *r.Description
	}
	if r.Timezone != nil {
		s.Timezone = *r.Timezone
	}
	if r.NotificationChannel != nil {
		s.NotificationChannel = *r.NotificationChannel
	}
}

// CreateLayerRequest is the request body for POST /api/v1/schedules/:id/layers.
type CreateLayerRequest struct {
	Name                 string               `json:"name"                   binding:"required,min=1,max=255"`
	OrderIndex           int                  `json:"order_index"            binding:"min=0"`
	RotationType         models.RotationType  `json:"rotation_type"          binding:"required,oneof=daily weekly custom"`
	RotationStart        *time.Time           `json:"rotation_start"`        // nil = default to midnight UTC today
	ShiftDurationSeconds *int                 `json:"shift_duration_seconds" binding:"omitempty,min=1"`
	Participants         []ParticipantRequest `json:"participants"           binding:"required,min=1,dive"`
}

// ParticipantRequest is an inline participant within a CreateLayerRequest.
type ParticipantRequest struct {
	UserName   string `json:"user_name"   binding:"required,min=1,max=255"`
	OrderIndex int    `json:"order_index" binding:"min=0"`
}

// ToLayer converts CreateLayerRequest to models.ScheduleLayer (without participants).
// scheduleID must be set by the handler.
func (r *CreateLayerRequest) ToLayer(scheduleID uuid.UUID) *models.ScheduleLayer {
	rotationStart := midnightUTC()
	if r.RotationStart != nil {
		rotationStart = *r.RotationStart
	}

	shiftDuration := defaultShiftDuration(r.RotationType)
	if r.ShiftDurationSeconds != nil {
		shiftDuration = *r.ShiftDurationSeconds
	}

	return &models.ScheduleLayer{
		ScheduleID:           scheduleID,
		Name:                 r.Name,
		OrderIndex:           r.OrderIndex,
		RotationType:         r.RotationType,
		RotationStart:        rotationStart,
		ShiftDurationSeconds: shiftDuration,
	}
}

// ToParticipants converts ParticipantRequest slice to models.ScheduleParticipant slice.
func (r *CreateLayerRequest) ToParticipants(layerID uuid.UUID) []models.ScheduleParticipant {
	participants := make([]models.ScheduleParticipant, len(r.Participants))
	for i, p := range r.Participants {
		participants[i] = models.ScheduleParticipant{
			LayerID:    layerID,
			UserName:   p.UserName,
			OrderIndex: p.OrderIndex,
		}
	}
	return participants
}

// UpdateLayerRequest is the request body for PATCH /api/v1/schedules/:id/layers/:layer_id.
// All fields are optional; nil means "no change". Participants, if provided (even as an
// empty slice), replaces the full participant list atomically.
type UpdateLayerRequest struct {
	Name                 *string              `json:"name"                   binding:"omitempty,min=1,max=255"`
	RotationType         *models.RotationType `json:"rotation_type"          binding:"omitempty,oneof=daily weekly custom"`
	RotationStart        *time.Time           `json:"rotation_start"`
	ShiftDurationSeconds *int                 `json:"shift_duration_seconds" binding:"omitempty,min=1"`
	// If non-nil, replaces all participants. nil = no change. [] = clear all.
	Participants []ParticipantRequest `json:"participants"`
}

// CreateOverrideRequest is the request body for POST /api/v1/schedules/:id/overrides.
type CreateOverrideRequest struct {
	OverrideUser string    `json:"override_user" binding:"required,min=1,max=255"`
	StartTime    time.Time `json:"start_time"    binding:"required"`
	EndTime      time.Time `json:"end_time"      binding:"required"`
	CreatedBy    string    `json:"created_by"    binding:"max=255"`
}

// ToModel converts CreateOverrideRequest to models.ScheduleOverride.
func (r *CreateOverrideRequest) ToModel(scheduleID uuid.UUID) *models.ScheduleOverride {
	createdBy := r.CreatedBy
	if createdBy == "" {
		createdBy = "api"
	}
	return &models.ScheduleOverride{
		ScheduleID:   scheduleID,
		OverrideUser: r.OverrideUser,
		StartTime:    r.StartTime,
		EndTime:      r.EndTime,
		CreatedBy:    createdBy,
	}
}

// midnightUTC returns midnight UTC for today.
func midnightUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// defaultShiftDuration returns the canonical shift length in seconds for a rotation type.
func defaultShiftDuration(rt models.RotationType) int {
	switch rt {
	case models.RotationTypeDaily:
		return 86400
	case models.RotationTypeWeekly:
		return 604800
	default: // custom — caller must provide shift_duration_seconds
		return 604800
	}
}

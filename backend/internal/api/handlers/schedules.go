package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
)

// ListSchedules handles GET /api/v1/schedules
func ListSchedules(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		schedules, err := repo.GetAll()
		if err != nil {
			slog.Error("failed to list schedules", "error", err, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}
		responses := make([]dto.ScheduleResponse, len(schedules))
		for i := range schedules {
			responses[i] = dto.ToScheduleResponse(&schedules[i])
		}
		c.JSON(http.StatusOK, gin.H{"data": responses, "total": len(responses)})
	}
}

// GetSchedule handles GET /api/v1/schedules/:id
// Returns the schedule with all layers and participants embedded.
func GetSchedule(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid schedule ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}
		schedule, err := repo.GetWithLayers(id)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule", id.String())
				return
			}
			slog.Error("failed to get schedule", "error", err, "id", id, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToScheduleResponse(schedule))
	}
}

// CreateSchedule handles POST /api/v1/schedules
func CreateSchedule(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.CreateScheduleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}
		schedule := req.ToModel()
		if err := repo.Create(schedule); err != nil {
			slog.Error("failed to create schedule", "error", err, "name", req.Name, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}
		slog.Info("schedule created", "id", schedule.ID, "name", schedule.Name, "request_id", c.GetString("request_id"))
		schedule.Layers = nil // no layers on create response; client can GET
		c.JSON(http.StatusCreated, dto.ToScheduleResponse(schedule))
	}
}

// UpdateSchedule handles PATCH /api/v1/schedules/:id
func UpdateSchedule(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid schedule ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}
		var req dto.UpdateScheduleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}
		schedule, err := repo.GetByID(id)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule", id.String())
				return
			}
			slog.Error("failed to get schedule for update", "error", err, "id", id, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}
		req.ApplyTo(schedule)
		if err := repo.Update(schedule); err != nil {
			slog.Error("failed to update schedule", "error", err, "id", id, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}
		slog.Info("schedule updated", "id", schedule.ID, "request_id", c.GetString("request_id"))
		c.JSON(http.StatusOK, dto.ToScheduleResponse(schedule))
	}
}

// DeleteSchedule handles DELETE /api/v1/schedules/:id
func DeleteSchedule(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid schedule ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}
		if err := repo.Delete(id); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule", id.String())
				return
			}
			slog.Error("failed to delete schedule", "error", err, "id", id, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}
		slog.Info("schedule deleted", "id", id, "request_id", c.GetString("request_id"))
		c.JSON(http.StatusNoContent, nil)
	}
}

// CreateLayer handles POST /api/v1/schedules/:id/layers
// Accepts the layer definition plus an inline participants array.
// Participants are bulk-inserted after the layer is created.
func CreateLayer(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheduleID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid schedule ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}
		// Verify schedule exists
		if _, err := repo.GetByID(scheduleID); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule", scheduleID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}

		var req dto.CreateLayerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Validate: custom rotation type requires explicit shift_duration_seconds
		if req.RotationType == models.RotationTypeCustom && req.ShiftDurationSeconds == nil {
			dto.BadRequest(c, "shift_duration_seconds is required for custom rotation type", map[string]interface{}{
				"rotation_type":          "custom",
				"shift_duration_seconds": "must be provided when rotation_type is custom",
			})
			return
		}

		layer := req.ToLayer(scheduleID)
		if err := repo.CreateLayer(layer); err != nil {
			slog.Error("failed to create layer", "error", err, "schedule_id", scheduleID, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}

		// Bulk-insert participants
		participants := req.ToParticipants(layer.ID)
		if len(participants) > 0 {
			if err := repo.CreateParticipantsBulk(participants); err != nil {
				slog.Error("failed to create participants", "error", err, "layer_id", layer.ID, "request_id", c.GetString("request_id"))
				dto.InternalError(c, err)
				return
			}
			layer.Participants = participants
		}

		slog.Info("schedule layer created", "layer_id", layer.ID, "schedule_id", scheduleID, "request_id", c.GetString("request_id"))
		c.JSON(http.StatusCreated, dto.ToLayerResponse(layer))
	}
}

// DeleteLayer handles DELETE /api/v1/schedules/:id/layers/:layer_id
func DeleteLayer(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		layerID, err := uuid.Parse(c.Param("layer_id"))
		if err != nil {
			dto.BadRequest(c, "Invalid layer ID", map[string]interface{}{"layer_id": "must be a valid UUID"})
			return
		}
		if err := repo.DeleteLayer(layerID); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule_layer", layerID.String())
				return
			}
			slog.Error("failed to delete layer", "error", err, "layer_id", layerID, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}
		slog.Info("schedule layer deleted", "layer_id", layerID, "request_id", c.GetString("request_id"))
		c.JSON(http.StatusNoContent, nil)
	}
}

// GetOnCall handles GET /api/v1/schedules/:id/oncall?at=<RFC3339>
// If `at` is omitted, uses time.Now().
func GetOnCall(repo repository.ScheduleRepository, evaluator services.ScheduleEvaluator) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheduleID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid schedule ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}

		at := time.Now().UTC()
		if atStr := c.Query("at"); atStr != "" {
			at, err = time.Parse(time.RFC3339, atStr)
			if err != nil {
				dto.BadRequest(c, "Invalid `at` parameter", map[string]interface{}{"at": "must be RFC3339, e.g. 2006-01-02T15:04:05Z"})
				return
			}
		}

		user, err := evaluator.WhoIsOnCall(scheduleID, at)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule", scheduleID.String())
				return
			}
			slog.Error("failed to evaluate on-call", "error", err, "schedule_id", scheduleID, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}

		// Determine if override was active (re-check overrides for the is_override flag)
		overrides, _ := repo.GetActiveOverrides(scheduleID, at)
		isOverride := len(overrides) > 0

		c.JSON(http.StatusOK, dto.OnCallResponse{
			ScheduleID: scheduleID,
			At:         at,
			UserName:   user,
			IsOverride: isOverride,
		})
	}
}

// GetOnCallTimeline handles GET /api/v1/schedules/:id/oncall/timeline?from=&to=
// Both `from` and `to` are optional RFC3339 strings; default window is next 7 days.
func GetOnCallTimeline(evaluator services.ScheduleEvaluator) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheduleID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid schedule ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}

		var from, to time.Time
		if fromStr := c.Query("from"); fromStr != "" {
			from, err = time.Parse(time.RFC3339, fromStr)
			if err != nil {
				dto.BadRequest(c, "Invalid `from` parameter", map[string]interface{}{"from": "must be RFC3339"})
				return
			}
		}
		if toStr := c.Query("to"); toStr != "" {
			to, err = time.Parse(time.RFC3339, toStr)
			if err != nil {
				dto.BadRequest(c, "Invalid `to` parameter", map[string]interface{}{"to": "must be RFC3339"})
				return
			}
		}

		segments, err := evaluator.GetTimeline(scheduleID, from, to)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule", scheduleID.String())
				return
			}
			slog.Error("failed to get timeline", "error", err, "schedule_id", scheduleID, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}

		// Compute the actual window used (for the response envelope)
		now := time.Now().UTC()
		if from.IsZero() {
			from = now
		}
		if to.IsZero() {
			to = now.Add(7 * 24 * time.Hour)
		}

		c.JSON(http.StatusOK, dto.TimelineResponse{
			ScheduleID: scheduleID,
			From:       from,
			To:         to,
			Segments:   segments,
		})
	}
}

// CreateOverride handles POST /api/v1/schedules/:id/overrides
func CreateOverride(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheduleID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid schedule ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}
		if _, err := repo.GetByID(scheduleID); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule", scheduleID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}

		var req dto.CreateOverrideRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		override := req.ToModel(scheduleID)
		if err := repo.CreateOverride(override); err != nil {
			slog.Error("failed to create override", "error", err, "schedule_id", scheduleID, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}

		slog.Info("schedule override created", "override_id", override.ID, "schedule_id", scheduleID, "request_id", c.GetString("request_id"))
		c.JSON(http.StatusCreated, dto.ToOverrideResponse(override))
	}
}

// DeleteOverride handles DELETE /api/v1/schedules/:id/overrides/:override_id
func DeleteOverride(repo repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		overrideID, err := uuid.Parse(c.Param("override_id"))
		if err != nil {
			dto.BadRequest(c, "Invalid override ID", map[string]interface{}{"override_id": "must be a valid UUID"})
			return
		}
		if err := repo.DeleteOverride(overrideID); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "schedule_override", overrideID.String())
				return
			}
			slog.Error("failed to delete override", "error", err, "override_id", overrideID, "request_id", c.GetString("request_id"))
			dto.InternalError(c, err)
			return
		}
		slog.Info("schedule override deleted", "override_id", overrideID, "request_id", c.GetString("request_id"))
		c.JSON(http.StatusNoContent, nil)
	}
}

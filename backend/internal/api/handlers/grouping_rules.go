package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"gorm.io/gorm"
)

// ListGroupingRules handles GET /api/v1/grouping-rules
func ListGroupingRules(ruleRepo repository.GroupingRuleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse query parameter: enabled filter (optional)
		enabledFilter := c.Query("enabled")

		var rules []models.GroupingRule
		var err error

		switch enabledFilter {
		case "true":
			rules, err = ruleRepo.GetEnabled()
		case "false":
			// Get all rules and filter out enabled ones
			allRules, getAllErr := ruleRepo.GetAll()
			if getAllErr != nil {
				err = getAllErr
			} else {
				rules = []models.GroupingRule{}
				for _, rule := range allRules {
					if !rule.Enabled {
						rules = append(rules, rule)
					}
				}
			}
		default:
			// Get all rules
			rules, err = ruleRepo.GetAll()
		}

		if err != nil {
			slog.Error("failed to list grouping rules",
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		// Convert to response DTOs
		responses := make([]dto.GroupingRuleResponse, len(rules))
		for i, rule := range rules {
			responses[i] = dto.ToGroupingRuleResponse(&rule)
		}

		c.JSON(http.StatusOK, gin.H{
			"data":  responses,
			"total": len(responses),
		})
	}
}

// GetGroupingRule handles GET /api/v1/grouping-rules/:id
func GetGroupingRule(ruleRepo repository.GroupingRuleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")

		// Parse UUID
		id, err := uuid.Parse(idParam)
		if err != nil {
			dto.BadRequest(c, "Invalid grouping rule ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}

		// Fetch rule
		rule, err := ruleRepo.GetByID(id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				dto.NotFound(c, "grouping_rule", idParam)
				return
			}
			slog.Error("failed to get grouping rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.ToGroupingRuleResponse(rule))
	}
}

// CreateGroupingRule handles POST /api/v1/grouping-rules
// onRuleMutate is called after successful creation to invalidate caches (e.g., grouping engine)
func CreateGroupingRule(ruleRepo repository.GroupingRuleRepository, onRuleMutate func()) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.CreateGroupingRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Check for priority conflicts
		conflict, err := ruleRepo.CheckPriorityConflict(req.Priority, uuid.Nil)
		if err != nil {
			slog.Error("failed to check priority conflict",
				"error", err,
				"priority", req.Priority,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}
		if conflict != nil {
			dto.Conflict(c, "grouping rule priority already in use", map[string]interface{}{
				"priority":       req.Priority,
				"conflicting_id": conflict.ID,
				"conflicting_name": conflict.Name,
			})
			return
		}

		// Convert to model and create
		rule := req.ToModel()
		if err := ruleRepo.Create(rule); err != nil {
			slog.Error("failed to create grouping rule",
				"error", err,
				"name", req.Name,
				"priority", req.Priority,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		slog.Info("grouping rule created",
			"id", rule.ID,
			"name", rule.Name,
			"priority", rule.Priority,
			"enabled", rule.Enabled,
			"request_id", c.GetString("request_id"),
		)

		// Invalidate grouping engine cache so new rule takes effect immediately
		if onRuleMutate != nil {
			onRuleMutate()
		}

		c.JSON(http.StatusCreated, dto.ToGroupingRuleResponse(rule))
	}
}

// UpdateGroupingRule handles PUT /api/v1/grouping-rules/:id
// onRuleMutate is called after successful update to invalidate caches
func UpdateGroupingRule(ruleRepo repository.GroupingRuleRepository, onRuleMutate func()) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")

		// Parse UUID
		id, err := uuid.Parse(idParam)
		if err != nil {
			dto.BadRequest(c, "Invalid grouping rule ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}

		// Parse request body
		var req dto.UpdateGroupingRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Fetch existing rule
		rule, err := ruleRepo.GetByID(id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				dto.NotFound(c, "grouping_rule", idParam)
				return
			}
			slog.Error("failed to get grouping rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		// Check for priority conflicts if priority is being changed
		if req.Priority != nil && *req.Priority != rule.Priority {
			conflict, err := ruleRepo.CheckPriorityConflict(*req.Priority, id)
			if err != nil {
				slog.Error("failed to check priority conflict",
					"error", err,
					"priority", *req.Priority,
					"request_id", c.GetString("request_id"),
				)
				dto.InternalError(c, err)
				return
			}
			if conflict != nil {
				dto.Conflict(c, "grouping rule priority already in use", map[string]interface{}{
					"priority":         *req.Priority,
					"conflicting_id":   conflict.ID,
					"conflicting_name": conflict.Name,
				})
				return
			}
		}

		// Apply updates
		req.ApplyTo(rule)

		// Save changes
		if err := ruleRepo.Update(rule); err != nil {
			slog.Error("failed to update grouping rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		slog.Info("grouping rule updated",
			"id", rule.ID,
			"name", rule.Name,
			"priority", rule.Priority,
			"enabled", rule.Enabled,
			"request_id", c.GetString("request_id"),
		)

		// Invalidate grouping engine cache so changes take effect immediately
		if onRuleMutate != nil {
			onRuleMutate()
		}

		c.JSON(http.StatusOK, dto.ToGroupingRuleResponse(rule))
	}
}

// DeleteGroupingRule handles DELETE /api/v1/grouping-rules/:id
// onRuleMutate is called after successful deletion to invalidate caches
func DeleteGroupingRule(ruleRepo repository.GroupingRuleRepository, onRuleMutate func()) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")

		// Parse UUID
		id, err := uuid.Parse(idParam)
		if err != nil {
			dto.BadRequest(c, "Invalid grouping rule ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}

		// Check if rule exists
		_, err = ruleRepo.GetByID(id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				dto.NotFound(c, "grouping_rule", idParam)
				return
			}
			slog.Error("failed to get grouping rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		// Delete rule
		if err := ruleRepo.Delete(id); err != nil {
			slog.Error("failed to delete grouping rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		slog.Info("grouping rule deleted",
			"id", id,
			"request_id", c.GetString("request_id"),
		)

		// Invalidate grouping engine cache
		if onRuleMutate != nil {
			onRuleMutate()
		}

		c.JSON(http.StatusNoContent, nil)
	}
}

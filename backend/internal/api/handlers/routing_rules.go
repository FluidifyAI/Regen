package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
)

// ListRoutingRules handles GET /api/v1/routing-rules
func ListRoutingRules(ruleRepo repository.RoutingRuleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		enabledFilter := c.Query("enabled")

		var rules []models.RoutingRule
		var err error

		switch enabledFilter {
		case "true":
			rules, err = ruleRepo.GetEnabled()
		case "false":
			allRules, getAllErr := ruleRepo.GetAll()
			if getAllErr != nil {
				err = getAllErr
			} else {
				rules = []models.RoutingRule{}
				for _, rule := range allRules {
					if !rule.Enabled {
						rules = append(rules, rule)
					}
				}
			}
		default:
			rules, err = ruleRepo.GetAll()
		}

		if err != nil {
			slog.Error("failed to list routing rules",
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		responses := make([]dto.RoutingRuleResponse, len(rules))
		for i, rule := range rules {
			responses[i] = dto.ToRoutingRuleResponse(&rule)
		}

		c.JSON(http.StatusOK, gin.H{
			"data":  responses,
			"total": len(responses),
		})
	}
}

// GetRoutingRule handles GET /api/v1/routing-rules/:id
func GetRoutingRule(ruleRepo repository.RoutingRuleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid routing rule ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}

		rule, err := ruleRepo.GetByID(id)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "routing_rule", id.String())
				return
			}
			slog.Error("failed to get routing rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.ToRoutingRuleResponse(rule))
	}
}

// CreateRoutingRule handles POST /api/v1/routing-rules
// onRuleMutate is called after successful creation to invalidate the routing engine cache.
func CreateRoutingRule(ruleRepo repository.RoutingRuleRepository, onRuleMutate func()) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.CreateRoutingRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

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
			dto.Conflict(c, "routing rule priority already in use", map[string]interface{}{
				"priority":         req.Priority,
				"conflicting_id":   conflict.ID,
				"conflicting_name": conflict.Name,
			})
			return
		}

		rule := req.ToModel()
		if err := ruleRepo.Create(rule); err != nil {
			slog.Error("failed to create routing rule",
				"error", err,
				"name", req.Name,
				"priority", req.Priority,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		slog.Info("routing rule created",
			"id", rule.ID,
			"name", rule.Name,
			"priority", rule.Priority,
			"enabled", rule.Enabled,
			"request_id", c.GetString("request_id"),
		)

		if onRuleMutate != nil {
			onRuleMutate()
		}

		c.JSON(http.StatusCreated, dto.ToRoutingRuleResponse(rule))
	}
}

// UpdateRoutingRule handles PATCH /api/v1/routing-rules/:id
// onRuleMutate is called after successful update to invalidate the routing engine cache.
func UpdateRoutingRule(ruleRepo repository.RoutingRuleRepository, onRuleMutate func()) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid routing rule ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}

		var req dto.UpdateRoutingRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		rule, err := ruleRepo.GetByID(id)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "routing_rule", id.String())
				return
			}
			slog.Error("failed to get routing rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

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
				dto.Conflict(c, "routing rule priority already in use", map[string]interface{}{
					"priority":         *req.Priority,
					"conflicting_id":   conflict.ID,
					"conflicting_name": conflict.Name,
				})
				return
			}
		}

		req.ApplyTo(rule)

		if err := ruleRepo.Update(rule); err != nil {
			slog.Error("failed to update routing rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		slog.Info("routing rule updated",
			"id", rule.ID,
			"name", rule.Name,
			"priority", rule.Priority,
			"enabled", rule.Enabled,
			"request_id", c.GetString("request_id"),
		)

		if onRuleMutate != nil {
			onRuleMutate()
		}

		c.JSON(http.StatusOK, dto.ToRoutingRuleResponse(rule))
	}
}

// DeleteRoutingRule handles DELETE /api/v1/routing-rules/:id
// onRuleMutate is called after successful deletion to invalidate the routing engine cache.
func DeleteRoutingRule(ruleRepo repository.RoutingRuleRepository, onRuleMutate func()) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid routing rule ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}

		if err := ruleRepo.Delete(id); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "routing_rule", id.String())
				return
			}
			slog.Error("failed to delete routing rule",
				"error", err,
				"id", id,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		slog.Info("routing rule deleted",
			"id", id,
			"request_id", c.GetString("request_id"),
		)

		if onRuleMutate != nil {
			onRuleMutate()
		}

		c.JSON(http.StatusNoContent, nil)
	}
}


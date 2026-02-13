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

// ListEscalationPolicies handles GET /api/v1/escalation-policies
func ListEscalationPolicies(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// GetAllPoliciesWithTiers uses two queries (one policy list + one IN query
		// for all tiers) to avoid an N+1 pattern.
		policies, err := repo.GetAllPoliciesWithTiers()
		if err != nil {
			slog.Error("failed to list escalation policies", "error", err)
			dto.InternalError(c, err)
			return
		}

		responses := make([]dto.EscalationPolicyResponse, 0, len(policies))
		for i := range policies {
			responses = append(responses, dto.ToEscalationPolicyResponse(&policies[i]))
		}

		c.JSON(http.StatusOK, gin.H{
			"data":  responses,
			"total": len(responses),
		})
	}
}

// GetEscalationPolicy handles GET /api/v1/escalation-policies/:id
func GetEscalationPolicy(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid policy ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}

		policy, err := repo.GetPolicyWithTiers(id)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "escalation_policy", id.String())
				return
			}
			slog.Error("failed to get escalation policy", "id", id, "error", err)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.ToEscalationPolicyResponse(policy))
	}
}

// CreateEscalationPolicy handles POST /api/v1/escalation-policies
func CreateEscalationPolicy(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.CreateEscalationPolicyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		policy := &models.EscalationPolicy{
			ID:          uuid.New(),
			Name:        req.Name,
			Description: req.Description,
			Enabled:     enabled,
		}

		if err := repo.CreatePolicy(policy); err != nil {
			slog.Error("failed to create escalation policy", "error", err)
			dto.InternalError(c, err)
			return
		}

		policy.Tiers = []models.EscalationTier{}
		c.JSON(http.StatusCreated, dto.ToEscalationPolicyResponse(policy))
	}
}

// UpdateEscalationPolicy handles PATCH /api/v1/escalation-policies/:id
func UpdateEscalationPolicy(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid policy ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}

		var req dto.UpdateEscalationPolicyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		policy, err := repo.GetPolicyWithTiers(id)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "escalation_policy", id.String())
				return
			}
			slog.Error("failed to get escalation policy for update", "id", id, "error", err)
			dto.InternalError(c, err)
			return
		}

		if req.Name != nil {
			policy.Name = *req.Name
		}
		if req.Description != nil {
			policy.Description = *req.Description
		}
		if req.Enabled != nil {
			policy.Enabled = *req.Enabled
		}

		if err := repo.UpdatePolicy(policy); err != nil {
			slog.Error("failed to update escalation policy", "id", id, "error", err)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.ToEscalationPolicyResponse(policy))
	}
}

// DeleteEscalationPolicy handles DELETE /api/v1/escalation-policies/:id
func DeleteEscalationPolicy(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid policy ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}

		if err := repo.DeletePolicy(id); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "escalation_policy", id.String())
				return
			}
			slog.Error("failed to delete escalation policy", "id", id, "error", err)
			dto.InternalError(c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// CreateEscalationTier handles POST /api/v1/escalation-policies/:id/tiers
func CreateEscalationTier(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		policyID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid policy ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}

		var req dto.CreateEscalationTierRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Determine the next tier_index
		existingTiers, err := repo.GetTiersByPolicy(policyID)
		if err != nil {
			slog.Error("failed to get tiers for policy", "policy_id", policyID, "error", err)
			dto.InternalError(c, err)
			return
		}

		userNames := req.UserNames
		if userNames == nil {
			userNames = []string{}
		}

		tier := &models.EscalationTier{
			ID:             uuid.New(),
			PolicyID:       policyID,
			TierIndex:      len(existingTiers),
			TimeoutSeconds: req.TimeoutSeconds,
			TargetType:     models.EscalationTargetType(req.TargetType),
			ScheduleID:     req.ScheduleID,
			UserNames:      models.JSONBArray(userNames),
		}

		if err := repo.CreateTier(tier); err != nil {
			slog.Error("failed to create escalation tier", "policy_id", policyID, "error", err)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusCreated, dto.ToEscalationTierResponse(tier))
	}
}

// UpdateEscalationTier handles PATCH /api/v1/escalation-policies/:id/tiers/:tier_id
func UpdateEscalationTier(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		policyID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid policy ID", map[string]interface{}{"id": "must be a valid UUID"})
			return
		}

		tierID, err := uuid.Parse(c.Param("tier_id"))
		if err != nil {
			dto.BadRequest(c, "Invalid tier ID", map[string]interface{}{"tier_id": "must be a valid UUID"})
			return
		}

		var req dto.UpdateEscalationTierRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		// Fetch the tier to update
		tiers, err := repo.GetTiersByPolicy(policyID)
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		var tier *models.EscalationTier
		for i := range tiers {
			if tiers[i].ID == tierID {
				tier = &tiers[i]
				break
			}
		}
		if tier == nil {
			dto.NotFound(c, "escalation_tier", tierID.String())
			return
		}

		if req.TimeoutSeconds != nil {
			tier.TimeoutSeconds = *req.TimeoutSeconds
		}
		if req.TargetType != nil {
			tier.TargetType = models.EscalationTargetType(*req.TargetType)
		}
		if req.ScheduleID != nil {
			tier.ScheduleID = req.ScheduleID
		}
		if req.UserNames != nil {
			tier.UserNames = models.JSONBArray(req.UserNames)
		}

		if err := repo.UpdateTier(tier); err != nil {
			slog.Error("failed to update escalation tier", "tier_id", tierID, "error", err)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.ToEscalationTierResponse(tier))
	}
}

// DeleteEscalationTier handles DELETE /api/v1/escalation-policies/:id/tiers/:tier_id
func DeleteEscalationTier(repo repository.EscalationPolicyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		tierID, err := uuid.Parse(c.Param("tier_id"))
		if err != nil {
			dto.BadRequest(c, "Invalid tier ID", map[string]interface{}{"tier_id": "must be a valid UUID"})
			return
		}

		if err := repo.DeleteTier(tierID); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "escalation_tier", tierID.String())
				return
			}
			slog.Error("failed to delete escalation tier", "tier_id", tierID, "error", err)
			dto.InternalError(c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/api/handlers/dto"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/services"
)

// GetPostMortem handles GET /api/v1/incidents/:id/postmortem
func GetPostMortem(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		pm, err := pmSvc.GetPostMortem(incident.ID)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem", incident.ID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToPostMortemResponse(pm))
	}
}

// GeneratePostMortem handles POST /api/v1/incidents/:id/postmortem/generate
func GeneratePostMortem(incidentSvc services.IncidentService, pmSvc services.PostMortemService, aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !aiSvc.IsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"code":    "ai_not_configured",
					"message": "AI features are not configured. Set the OPENAI_API_KEY environment variable to enable them.",
				},
			})
			return
		}

		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}

		var req dto.GeneratePostMortemRequest
		// Body is optional — ignore bind error since template_id is optional
		_ = c.ShouldBindJSON(&req)

		pm, err := pmSvc.GeneratePostMortem(incident, req.TemplateID, "system")
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToPostMortemResponse(pm))
	}
}

// UpdatePostMortem handles PATCH /api/v1/incidents/:id/postmortem
func UpdatePostMortem(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}

		existing, err := pmSvc.GetPostMortem(incident.ID)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem", incident.ID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}

		var req dto.UpdatePostMortemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		var status *models.PostMortemStatus
		if req.Status != nil {
			s := models.PostMortemStatus(*req.Status)
			status = &s
		}

		pm, err := pmSvc.UpdatePostMortem(existing.ID, &services.UpdatePostMortemParams{
			Content: req.Content,
			Status:  status,
		})
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToPostMortemResponse(pm))
	}
}

// ExportPostMortem handles GET /api/v1/incidents/:id/postmortem/export
// Returns the post-mortem content as a downloadable Markdown file.
func ExportPostMortem(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		pm, err := pmSvc.GetPostMortem(incident.ID)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem", incident.ID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}

		filename := fmt.Sprintf("postmortem-INC-%d-%s.md", incident.IncidentNumber, time.Now().Format("2006-01-02"))
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(pm.Content))
	}
}

// CreateActionItem handles POST /api/v1/incidents/:id/postmortem/action-items
func CreateActionItem(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		pm, err := pmSvc.GetPostMortem(incident.ID)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem", incident.ID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}

		var req dto.CreateActionItemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		item, err := pmSvc.CreateActionItem(pm.ID, &services.CreateActionItemParams{
			Title:   req.Title,
			Owner:   req.Owner,
			DueDate: req.DueDate,
		})
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ToActionItemResponse(item))
	}
}

// UpdateActionItem handles PATCH /api/v1/incidents/:id/postmortem/action-items/:itemId
func UpdateActionItem(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate incident exists (keeps URL hierarchy consistent)
		_, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}

		itemID, err := uuid.Parse(c.Param("itemId"))
		if err != nil {
			dto.BadRequest(c, "Invalid action item ID", map[string]interface{}{
				"itemId": "must be a valid UUID",
			})
			return
		}

		var req dto.UpdateActionItemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}

		var status *models.ActionItemStatus
		if req.Status != nil {
			s := models.ActionItemStatus(*req.Status)
			status = &s
		}

		item, err := pmSvc.UpdateActionItem(itemID, &services.UpdateActionItemParams{
			Title:   req.Title,
			Owner:   req.Owner,
			DueDate: req.DueDate,
			Status:  status,
		})
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "action_item", itemID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToActionItemResponse(item))
	}
}

// DeleteActionItem handles DELETE /api/v1/incidents/:id/postmortem/action-items/:itemId
func DeleteActionItem(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}

		itemID, err := uuid.Parse(c.Param("itemId"))
		if err != nil {
			dto.BadRequest(c, "Invalid action item ID", map[string]interface{}{
				"itemId": "must be a valid UUID",
			})
			return
		}

		if err := pmSvc.DeleteActionItem(itemID); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "action_item", itemID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// CreatePostMortem handles POST /api/v1/incidents/:id/postmortem
// Creates a blank draft post-mortem for manual authoring.
func CreatePostMortem(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		pm, err := pmSvc.CreatePostMortem(incident.ID, "anonymous")
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ToPostMortemResponse(pm))
	}
}

// EnhancePostMortem handles POST /api/v1/incidents/:id/postmortem/enhance
// Takes existing content, runs it through AI to improve structure/clarity, saves as draft.
func EnhancePostMortem(incidentSvc services.IncidentService, pmSvc services.PostMortemService, aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !aiSvc.IsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"code":    "ai_not_configured",
					"message": "AI features are not configured.",
				},
			})
			return
		}
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		pm, err := pmSvc.GetPostMortem(incident.ID)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem", incident.ID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		var req dto.EnhancePostMortemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}
		enhanced, err := pmSvc.EnhancePostMortem(pm, req.Content)
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToPostMortemResponse(enhanced))
	}
}

// ListPostMortemComments handles GET /api/v1/incidents/:id/postmortem/comments
func ListPostMortemComments(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		pm, err := pmSvc.GetPostMortem(incident.ID)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem", incident.ID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		comments, err := pmSvc.ListComments(pm.ID)
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		resp := make([]dto.PostMortemCommentResponse, 0, len(comments))
		for i := range comments {
			resp = append(resp, dto.ToPostMortemCommentResponse(&comments[i]))
		}
		c.JSON(http.StatusOK, dto.ListCommentsResponse{Data: resp})
	}
}

// CreatePostMortemComment handles POST /api/v1/incidents/:id/postmortem/comments
func CreatePostMortemComment(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		pm, err := pmSvc.GetPostMortem(incident.ID)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem", incident.ID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		var req dto.CreateCommentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}
		comment, err := pmSvc.CreateComment(pm.ID, &services.CreateCommentParams{
			AuthorID:   "anonymous",
			AuthorName: req.AuthorName,
			Content:    req.Content,
		})
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ToPostMortemCommentResponse(comment))
	}
}

// DeletePostMortemComment handles DELETE /api/v1/incidents/:id/postmortem/comments/:commentId
func DeletePostMortemComment(incidentSvc services.IncidentService, pmSvc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		commentID, err := uuid.Parse(c.Param("commentId"))
		if err != nil {
			dto.BadRequest(c, "Invalid comment ID", map[string]interface{}{
				"commentId": "must be a valid UUID",
			})
			return
		}
		if err := pmSvc.DeleteComment(commentID); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem_comment", commentID.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}


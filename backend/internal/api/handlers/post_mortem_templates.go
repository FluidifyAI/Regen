package handlers

import (
	"net/http"

	"github.com/FluidifyAI/Regen/backend/internal/api/handlers/dto"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListPostMortemTemplates handles GET /api/v1/post-mortem-templates
func ListPostMortemTemplates(svc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		templates, err := svc.ListTemplates()
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		resp := make([]dto.PostMortemTemplateResponse, 0, len(templates))
		for i := range templates {
			resp = append(resp, dto.ToPostMortemTemplateResponse(&templates[i]))
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	}
}

// GetPostMortemTemplate handles GET /api/v1/post-mortem-templates/:id
func GetPostMortemTemplate(svc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid template ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}
		tmpl, err := svc.GetTemplate(id)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem_template", id.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToPostMortemTemplateResponse(tmpl))
	}
}

// CreatePostMortemTemplate handles POST /api/v1/post-mortem-templates
func CreatePostMortemTemplate(svc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.CreatePostMortemTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}
		tmpl, err := svc.CreateTemplate(&services.CreatePostMortemTemplateParams{
			Name:        req.Name,
			Description: req.Description,
			Sections:    req.Sections,
		})
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ToPostMortemTemplateResponse(tmpl))
	}
}

// UpdatePostMortemTemplate handles PATCH /api/v1/post-mortem-templates/:id
func UpdatePostMortemTemplate(svc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid template ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}
		var req dto.UpdatePostMortemTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.ValidationError(c, err)
			return
		}
		tmpl, err := svc.UpdateTemplate(id, &services.UpdatePostMortemTemplateParams{
			Name:        req.Name,
			Description: req.Description,
			Sections:    req.Sections,
		})
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem_template", id.String())
				return
			}
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToPostMortemTemplateResponse(tmpl))
	}
}

// DeletePostMortemTemplate handles DELETE /api/v1/post-mortem-templates/:id
func DeletePostMortemTemplate(svc services.PostMortemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			dto.BadRequest(c, "Invalid template ID", map[string]interface{}{
				"id": "must be a valid UUID",
			})
			return
		}
		if err := svc.DeleteTemplate(id); err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "post_mortem_template", id.String())
				return
			}
			if isBuiltInConflict(err) {
				dto.Conflict(c, err.Error(), nil)
				return
			}
			dto.InternalError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

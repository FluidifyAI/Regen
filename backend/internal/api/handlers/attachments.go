package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/FluidifyAI/Regen/backend/internal/api/handlers/dto"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxUploadSize = 10*1024*1024 + 512 // 10 MB + sniff buffer

// ListAttachments handles GET /api/v1/incidents/:id/attachments
func ListAttachments(incidentSvc services.IncidentService, attSvc services.AttachmentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		atts, err := attSvc.List(incident.ID)
		if err != nil {
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": dto.ToAttachmentListResponse(atts)})
	}
}

// UploadAttachment handles POST /api/v1/incidents/:id/attachments (multipart/form-data, field "file")
func UploadAttachment(incidentSvc services.IncidentService, attSvc services.AttachmentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)
		if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
			dto.BadRequest(c, "File too large or malformed multipart request", nil)
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			dto.BadRequest(c, "Missing 'file' field in multipart form", nil)
			return
		}
		defer file.Close()

		uploader := "system"
		if u, exists := c.Get("username"); exists {
			if s, ok := u.(string); ok && s != "" {
				uploader = s
			}
		}

		fileName := filepath.Base(strings.ReplaceAll(header.Filename, "\x00", ""))
		att, err := attSvc.Upload(incident.ID, fileName, uploader, file)
		if err != nil {
			if strings.Contains(err.Error(), "10 MB") {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": gin.H{"code": "file_too_large", "message": err.Error()},
				})
				return
			}
			if strings.Contains(err.Error(), "not allowed") {
				c.JSON(http.StatusUnsupportedMediaType, gin.H{
					"error": gin.H{"code": "unsupported_file_type", "message": err.Error()},
				})
				return
			}
			dto.InternalError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ToAttachmentResponse(att))
	}
}

// DownloadAttachment handles GET /api/v1/incidents/:id/attachments/:aid/download
func DownloadAttachment(incidentSvc services.IncidentService, attSvc services.AttachmentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		aidStr := c.Param("aid")
		aid, err := uuid.Parse(aidStr)
		if err != nil {
			dto.BadRequest(c, "Invalid attachment ID", nil)
			return
		}
		att, data, err := attSvc.Download(aid)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "attachment", aidStr)
				return
			}
			dto.InternalError(c, err)
			return
		}
		if att.IncidentID != incident.ID {
			dto.NotFound(c, "attachment", aidStr)
			return
		}
		c.Header("Content-Type", att.MimeType)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, att.FileName))
		c.Header("Content-Length", fmt.Sprintf("%d", len(data)))
		c.Data(http.StatusOK, att.MimeType, data)
	}
}

// DeleteAttachment handles DELETE /api/v1/incidents/:id/attachments/:aid
func DeleteAttachment(incidentSvc services.IncidentService, attSvc services.AttachmentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		incident, ok := resolveIncident(c, incidentSvc)
		if !ok {
			return
		}
		aidStr := c.Param("aid")
		aid, err := uuid.Parse(aidStr)
		if err != nil {
			dto.BadRequest(c, "Invalid attachment ID", nil)
			return
		}
		att, _, err := attSvc.Download(aid)
		if err != nil {
			if isNotFound(err) {
				dto.NotFound(c, "attachment", aidStr)
				return
			}
			dto.InternalError(c, err)
			return
		}
		if att.IncidentID != incident.ID {
			dto.NotFound(c, "attachment", aidStr)
			return
		}
		if err := attSvc.Delete(aid); err != nil {
			dto.InternalError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

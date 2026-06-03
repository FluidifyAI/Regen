package dto

import (
	"fmt"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

// AttachmentResponse is the API representation of an attachment (no binary data).
type AttachmentResponse struct {
	ID          uuid.UUID `json:"id"`
	IncidentID  uuid.UUID `json:"incident_id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	MimeType    string    `json:"mime_type"`
	UploadedBy  string    `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
	DownloadURL string    `json:"download_url"`
}

func ToAttachmentResponse(a *models.IncidentAttachment) AttachmentResponse {
	return AttachmentResponse{
		ID:          a.ID,
		IncidentID:  a.IncidentID,
		FileName:    a.FileName,
		FileSize:    a.FileSize,
		MimeType:    a.MimeType,
		UploadedBy:  a.UploadedBy,
		CreatedAt:   a.CreatedAt,
		DownloadURL: fmt.Sprintf("/api/v1/incidents/%s/attachments/%s/download", a.IncidentID, a.ID),
	}
}

func ToAttachmentListResponse(atts []models.IncidentAttachment) []AttachmentResponse {
	out := make([]AttachmentResponse, len(atts))
	for i := range atts {
		out[i] = ToAttachmentResponse(&atts[i])
	}
	return out
}

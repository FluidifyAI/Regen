package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IncidentAttachment holds metadata for a file attached to an incident.
// Binary content lives in IncidentAttachmentData to keep list queries fast.
type IncidentAttachment struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IncidentID uuid.UUID `gorm:"type:uuid;not null"                             json:"incident_id"`
	FileName   string    `gorm:"type:text;not null"                             json:"file_name"`
	FileSize   int64     `gorm:"not null"                                       json:"file_size"`
	MimeType   string    `gorm:"type:text;not null"                             json:"mime_type"`
	UploadedBy string    `gorm:"type:text;not null"                             json:"uploaded_by"`
	CreatedAt  time.Time `gorm:"not null;default:now()"                         json:"created_at"`
}

func (IncidentAttachment) TableName() string { return "incident_attachments" }

// BeforeCreate ensures the UUID primary key is populated before insertion,
// which is required for SQLite in tests (no gen_random_uuid() support).
func (a *IncidentAttachment) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// IncidentAttachmentData holds the raw bytes for an attachment.
// Stored separately so SELECT on incident_attachments never reads binary data.
type IncidentAttachmentData struct {
	AttachmentID uuid.UUID `gorm:"type:uuid;primaryKey" json:"attachment_id"`
	Data         []byte    `gorm:"type:bytea;not null"  json:"-"`
}

func (IncidentAttachmentData) TableName() string { return "incident_attachment_data" }

package repository

import (
	"errors"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AttachmentRepository manages incident file attachments.
type AttachmentRepository interface {
	Create(att *models.IncidentAttachment, data []byte) error
	ListByIncident(incidentID uuid.UUID) ([]models.IncidentAttachment, error)
	GetWithData(id uuid.UUID) (*models.IncidentAttachment, []byte, error)
	Delete(id uuid.UUID) error
}

type attachmentRepository struct {
	db *gorm.DB
}

func NewAttachmentRepository(db *gorm.DB) AttachmentRepository {
	return &attachmentRepository{db: db}
}

func (r *attachmentRepository) Create(att *models.IncidentAttachment, data []byte) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(att).Error; err != nil {
			return &DatabaseError{Op: "create attachment", Err: err}
		}
		ad := &models.IncidentAttachmentData{
			AttachmentID: att.ID,
			Data:         data,
		}
		if err := tx.Create(ad).Error; err != nil {
			return &DatabaseError{Op: "create attachment_data", Err: err}
		}
		return nil
	})
}

func (r *attachmentRepository) ListByIncident(incidentID uuid.UUID) ([]models.IncidentAttachment, error) {
	var atts []models.IncidentAttachment
	if err := r.db.Where("incident_id = ?", incidentID).Order("created_at ASC").Find(&atts).Error; err != nil {
		return nil, &DatabaseError{Op: "list attachments", Err: err}
	}
	return atts, nil
}

func (r *attachmentRepository) GetWithData(id uuid.UUID) (*models.IncidentAttachment, []byte, error) {
	var att models.IncidentAttachment
	if err := r.db.First(&att, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &NotFoundError{Resource: "attachment", ID: id.String()}
		}
		return nil, nil, &DatabaseError{Op: "get attachment", Err: err}
	}
	var ad models.IncidentAttachmentData
	if err := r.db.First(&ad, "attachment_id = ?", id).Error; err != nil {
		return nil, nil, &DatabaseError{Op: "get attachment_data", Err: err}
	}
	return &att, ad.Data, nil
}

func (r *attachmentRepository) Delete(id uuid.UUID) error {
	// CASCADE on incident_attachment_data handles the data row automatically.
	result := r.db.Delete(&models.IncidentAttachment{}, "id = ?", id)
	if result.Error != nil {
		return &DatabaseError{Op: "delete attachment", Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "attachment", ID: id.String()}
	}
	return nil
}

package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"gorm.io/gorm"
)

// PostMortemTemplateRepository provides data access for post-mortem templates.
type PostMortemTemplateRepository interface {
	List() ([]models.PostMortemTemplate, error)
	GetByID(id uuid.UUID) (*models.PostMortemTemplate, error)
	Create(tmpl *models.PostMortemTemplate) error
	Update(tmpl *models.PostMortemTemplate) error
	Delete(id uuid.UUID) error
}

type postMortemTemplateRepository struct {
	db *gorm.DB
}

func NewPostMortemTemplateRepository(db *gorm.DB) PostMortemTemplateRepository {
	return &postMortemTemplateRepository{db: db}
}

func (r *postMortemTemplateRepository) List() ([]models.PostMortemTemplate, error) {
	var templates []models.PostMortemTemplate
	if err := r.db.Order("is_built_in DESC, name ASC").Find(&templates).Error; err != nil {
		return nil, &DatabaseError{Op: "list post_mortem_templates", Err: err}
	}
	return templates, nil
}

func (r *postMortemTemplateRepository) GetByID(id uuid.UUID) (*models.PostMortemTemplate, error) {
	var tmpl models.PostMortemTemplate
	if err := r.db.First(&tmpl, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "post_mortem_template", ID: id.String()}
		}
		return nil, &DatabaseError{Op: "get post_mortem_template by id", Err: err}
	}
	return &tmpl, nil
}

func (r *postMortemTemplateRepository) Create(tmpl *models.PostMortemTemplate) error {
	if err := r.db.Create(tmpl).Error; err != nil {
		return &DatabaseError{Op: "create post_mortem_template", Err: err}
	}
	return nil
}

func (r *postMortemTemplateRepository) Update(tmpl *models.PostMortemTemplate) error {
	updates := map[string]interface{}{
		"name":        tmpl.Name,
		"description": tmpl.Description,
		"sections":    tmpl.Sections,
		"updated_at":  time.Now(),
	}
	if err := r.db.Model(tmpl).Updates(updates).Error; err != nil {
		return &DatabaseError{Op: "update post_mortem_template", Err: err}
	}
	return nil
}

func (r *postMortemTemplateRepository) Delete(id uuid.UUID) error {
	if err := r.db.Delete(&models.PostMortemTemplate{}, "id = ?", id).Error; err != nil {
		return &DatabaseError{Op: "delete post_mortem_template", Err: err}
	}
	return nil
}

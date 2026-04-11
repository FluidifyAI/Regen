package repository

import (
	"errors"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostMortemRepository provides data access for post-mortems and their action items.
type PostMortemRepository interface {
	GetByIncidentID(incidentID uuid.UUID) (*models.PostMortem, error)
	GetByID(id uuid.UUID) (*models.PostMortem, error)
	Create(pm *models.PostMortem) error
	// Upsert inserts pm or, if a post-mortem for the same incident already exists,
	// overwrites its AI-generated fields. Safe for concurrent callers.
	Upsert(pm *models.PostMortem) error
	Update(pm *models.PostMortem) error

	ListActionItems(postMortemID uuid.UUID) ([]models.ActionItem, error)
	GetActionItemByID(id uuid.UUID) (*models.ActionItem, error)
	CreateActionItem(item *models.ActionItem) error
	UpdateActionItem(item *models.ActionItem) error
	DeleteActionItem(id uuid.UUID) error
}

type postMortemRepository struct {
	db *gorm.DB
}

func NewPostMortemRepository(db *gorm.DB) PostMortemRepository {
	return &postMortemRepository{db: db}
}

func (r *postMortemRepository) GetByIncidentID(incidentID uuid.UUID) (*models.PostMortem, error) {
	var pm models.PostMortem
	if err := r.db.First(&pm, "incident_id = ?", incidentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "post_mortem", ID: incidentID.String()}
		}
		return nil, &DatabaseError{Op: "get post_mortem by incident_id", Err: err}
	}
	if err := r.db.Where("post_mortem_id = ?", pm.ID).Order("created_at ASC").Find(&pm.ActionItems).Error; err != nil {
		return nil, &DatabaseError{Op: "list action_items for post_mortem", Err: err}
	}
	return &pm, nil
}

func (r *postMortemRepository) GetByID(id uuid.UUID) (*models.PostMortem, error) {
	var pm models.PostMortem
	if err := r.db.First(&pm, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "post_mortem", ID: id.String()}
		}
		return nil, &DatabaseError{Op: "get post_mortem by id", Err: err}
	}
	if err := r.db.Where("post_mortem_id = ?", pm.ID).Order("created_at ASC").Find(&pm.ActionItems).Error; err != nil {
		return nil, &DatabaseError{Op: "list action_items for post_mortem", Err: err}
	}
	return &pm, nil
}

func (r *postMortemRepository) Create(pm *models.PostMortem) error {
	if err := r.db.Create(pm).Error; err != nil {
		return &DatabaseError{Op: "create post_mortem", Err: err}
	}
	return nil
}

// Upsert uses INSERT ... ON CONFLICT DO UPDATE so concurrent callers are safe.
// The unique index on (incident_id) is the conflict target.
func (r *postMortemRepository) Upsert(pm *models.PostMortem) error {
	err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "incident_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"content", "template_id", "template_name",
			"generated_by", "generated_at", "status",
			"published_at", "updated_at",
		}),
	}).Create(pm).Error
	if err != nil {
		return &DatabaseError{Op: "upsert post_mortem", Err: err}
	}
	return nil
}

func (r *postMortemRepository) Update(pm *models.PostMortem) error {
	updates := map[string]interface{}{
		"status":       pm.Status,
		"content":      pm.Content,
		"published_at": pm.PublishedAt, // always write — nil clears it when reverting to draft
		"updated_at":   time.Now(),
	}
	if pm.GeneratedAt != nil {
		updates["generated_at"] = pm.GeneratedAt
	}
	// Allow resetting template info on regeneration
	updates["template_id"] = pm.TemplateID
	updates["template_name"] = pm.TemplateName
	updates["generated_by"] = pm.GeneratedBy

	if err := r.db.Model(pm).Updates(updates).Error; err != nil {
		return &DatabaseError{Op: "update post_mortem", Err: err}
	}
	return nil
}

func (r *postMortemRepository) ListActionItems(postMortemID uuid.UUID) ([]models.ActionItem, error) {
	var items []models.ActionItem
	if err := r.db.Where("post_mortem_id = ?", postMortemID).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, &DatabaseError{Op: "list action_items", Err: err}
	}
	return items, nil
}

func (r *postMortemRepository) GetActionItemByID(id uuid.UUID) (*models.ActionItem, error) {
	var item models.ActionItem
	if err := r.db.First(&item, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "action_item", ID: id.String()}
		}
		return nil, &DatabaseError{Op: "get action_item by id", Err: err}
	}
	return &item, nil
}

func (r *postMortemRepository) CreateActionItem(item *models.ActionItem) error {
	if err := r.db.Create(item).Error; err != nil {
		return &DatabaseError{Op: "create action_item", Err: err}
	}
	return nil
}

func (r *postMortemRepository) UpdateActionItem(item *models.ActionItem) error {
	updates := map[string]interface{}{
		"title":      item.Title,
		"owner":      item.Owner,
		"due_date":   item.DueDate,
		"status":     item.Status,
		"updated_at": time.Now(),
	}
	if err := r.db.Model(item).Updates(updates).Error; err != nil {
		return &DatabaseError{Op: "update action_item", Err: err}
	}
	return nil
}

func (r *postMortemRepository) DeleteActionItem(id uuid.UUID) error {
	result := r.db.Delete(&models.ActionItem{}, "id = ?", id)
	if result.Error != nil {
		return &DatabaseError{Op: "delete action_item", Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "action_item", ID: id.String()}
	}
	return nil
}

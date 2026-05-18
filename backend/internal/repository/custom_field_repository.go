package repository

import (
	"fmt"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReorderItem struct {
	ID    uuid.UUID
	Order int
}

type CustomFieldRepository interface {
	List() ([]models.CustomFieldDefinition, error)
	Create(def *models.CustomFieldDefinition) error
	Update(def *models.CustomFieldDefinition) error
	Delete(id uuid.UUID) error
	Reorder(items []ReorderItem) error
	GetByKey(key string) (*models.CustomFieldDefinition, error)
	CountUsage(key string) (int64, error)
}

type customFieldRepository struct {
	db *gorm.DB
}

func NewCustomFieldRepository(db *gorm.DB) CustomFieldRepository {
	return &customFieldRepository{db: db}
}

func (r *customFieldRepository) List() ([]models.CustomFieldDefinition, error) {
	var defs []models.CustomFieldDefinition
	if err := r.db.Order("display_order ASC, created_at ASC").Find(&defs).Error; err != nil {
		return nil, err
	}
	return defs, nil
}

func (r *customFieldRepository) Create(def *models.CustomFieldDefinition) error {
	return r.db.Create(def).Error
}

func (r *customFieldRepository) Update(def *models.CustomFieldDefinition) error {
	return r.db.Save(def).Error
}

func (r *customFieldRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.CustomFieldDefinition{}, id).Error
}

func (r *customFieldRepository) Reorder(items []ReorderItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if err := tx.Model(&models.CustomFieldDefinition{}).
				Where("id = ?", item.ID).
				Update("display_order", item.Order).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *customFieldRepository) GetByKey(key string) (*models.CustomFieldDefinition, error) {
	var def models.CustomFieldDefinition
	if err := r.db.Where("key = ?", key).First(&def).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &NotFoundError{Resource: "custom_field", ID: key}
		}
		return nil, err
	}
	return &def, nil
}

func (r *customFieldRepository) CountUsage(key string) (int64, error) {
	var count int64
	var err error
	// Postgres uses the JSONB key-exists operator; SQLite (test env) uses json_extract.
	if r.db.Dialector.Name() == "postgres" {
		err = r.db.Raw("SELECT COUNT(*) FROM incidents WHERE custom_fields ? ?", key).Scan(&count).Error
	} else {
		err = r.db.Raw(
			"SELECT COUNT(*) FROM incidents WHERE json_extract(custom_fields, ?) IS NOT NULL",
			fmt.Sprintf("$.%s", key),
		).Scan(&count).Error
	}
	return count, err
}

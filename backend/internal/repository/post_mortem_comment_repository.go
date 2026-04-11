package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"gorm.io/gorm"
)

type PostMortemCommentRepository interface {
	ListByPostMortemID(postMortemID uuid.UUID) ([]models.PostMortemComment, error)
	Create(comment *models.PostMortemComment) error
	GetByID(id uuid.UUID) (*models.PostMortemComment, error)
	Delete(id uuid.UUID) error
}

type postMortemCommentRepository struct {
	db *gorm.DB
}

func NewPostMortemCommentRepository(db *gorm.DB) PostMortemCommentRepository {
	return &postMortemCommentRepository{db: db}
}

func (r *postMortemCommentRepository) ListByPostMortemID(postMortemID uuid.UUID) ([]models.PostMortemComment, error) {
	var comments []models.PostMortemComment
	if err := r.db.Where("post_mortem_id = ?", postMortemID).Order("created_at ASC").Find(&comments).Error; err != nil {
		return nil, &DatabaseError{Op: "list post_mortem_comments", Err: err}
	}
	return comments, nil
}

func (r *postMortemCommentRepository) Create(comment *models.PostMortemComment) error {
	if err := r.db.Create(comment).Error; err != nil {
		return &DatabaseError{Op: "create post_mortem_comment", Err: err}
	}
	return nil
}

func (r *postMortemCommentRepository) GetByID(id uuid.UUID) (*models.PostMortemComment, error) {
	var comment models.PostMortemComment
	if err := r.db.First(&comment, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "post_mortem_comment", ID: id.String()}
		}
		return nil, &DatabaseError{Op: "get post_mortem_comment", Err: err}
	}
	return &comment, nil
}

func (r *postMortemCommentRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&models.PostMortemComment{}, "id = ?", id)
	if result.Error != nil {
		return &DatabaseError{Op: "delete post_mortem_comment", Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "post_mortem_comment", ID: id.String()}
	}
	return nil
}

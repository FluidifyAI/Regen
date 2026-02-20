package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PostMortemTemplate is a reusable template that defines the sections included
// in an AI-generated post-mortem. Built-in templates are seeded at migration
// time; users may create, edit, or delete any template.
type PostMortemTemplate struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string     `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	Description string     `gorm:"type:text;not null;default:''" json:"description"`
	Sections    JSONBArray `gorm:"type:jsonb;not null;default:'[]'" json:"sections"`
	IsBuiltIn   bool       `gorm:"not null;default:false" json:"is_built_in"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (PostMortemTemplate) TableName() string { return "post_mortem_templates" }

func (t *PostMortemTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

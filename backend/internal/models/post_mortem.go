package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PostMortemStatus represents the lifecycle state of a post-mortem document.
type PostMortemStatus string

const (
	PostMortemStatusDraft     PostMortemStatus = "draft"
	PostMortemStatusPublished PostMortemStatus = "published"
)

// ActionItemStatus represents the completion state of an action item.
type ActionItemStatus string

const (
	ActionItemStatusOpen       ActionItemStatus = "open"
	ActionItemStatusInProgress ActionItemStatus = "in_progress"
	ActionItemStatusClosed     ActionItemStatus = "closed"
)

// PostMortem is the AI-generated (or manually authored) post-mortem document
// for an incident. One per incident, enforced by a unique index.
// Content is stored as Markdown and is fully editable after generation.
type PostMortem struct {
	ID           uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IncidentID   uuid.UUID        `gorm:"type:uuid;not null;uniqueIndex:idx_post_mortems_incident_id" json:"incident_id"`
	TemplateID   *uuid.UUID       `gorm:"type:uuid" json:"template_id,omitempty"`
	TemplateName string           `gorm:"type:varchar(100);not null;default:'Standard'" json:"template_name"`
	Status       PostMortemStatus `gorm:"type:varchar(20);not null;default:'draft'" json:"status"`
	Content      string           `gorm:"type:text;not null;default:''" json:"content"`
	GeneratedBy  string           `gorm:"type:varchar(20);not null;default:'ai'" json:"generated_by"`
	GeneratedAt  *time.Time       `json:"generated_at,omitempty"`
	PublishedAt  *time.Time       `json:"published_at,omitempty"`
	CreatedByID  string           `gorm:"type:varchar(255);not null;default:'system'" json:"created_by_id"`
	CreatedAt    time.Time        `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time        `gorm:"not null;default:now()" json:"updated_at"`

	// Relationships (loaded explicitly, not by GORM auto-join)
	ActionItems []ActionItem `gorm:"-" json:"action_items,omitempty"`
}

func (PostMortem) TableName() string { return "post_mortems" }

func (p *PostMortem) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ActionItem is a follow-up task extracted from a post-mortem.
// Cascade-deleted when its parent post-mortem is deleted.
type ActionItem struct {
	ID           uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	PostMortemID uuid.UUID        `gorm:"type:uuid;not null;index" json:"post_mortem_id"`
	Title        string           `gorm:"type:varchar(500);not null" json:"title"`
	Owner        *string          `gorm:"type:varchar(255)" json:"owner,omitempty"`
	DueDate      *time.Time       `json:"due_date,omitempty"`
	Status       ActionItemStatus `gorm:"type:varchar(20);not null;default:'open'" json:"status"`
	CreatedAt    time.Time        `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time        `gorm:"not null;default:now()" json:"updated_at"`
}

func (ActionItem) TableName() string { return "action_items" }

func (a *ActionItem) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

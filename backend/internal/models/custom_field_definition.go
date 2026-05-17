package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	FieldTypeString   = "string"
	FieldTypeNumber   = "number"
	FieldTypeDropdown = "dropdown"
)

type DropdownOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type CustomFieldDefinition struct {
	ID           uuid.UUID       `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name         string          `gorm:"not null"                                       json:"name"`
	Key          string          `gorm:"uniqueIndex;not null"                           json:"key"`
	FieldType    string          `gorm:"column:field_type;not null"                     json:"field_type"`
	Options      json.RawMessage `gorm:"type:jsonb;default:'[]'"                        json:"options"`
	DisplayOrder int             `gorm:"default:0"                                      json:"display_order"`
	CreatedAt    time.Time       `                                                      json:"created_at"`
	UpdatedAt    time.Time       `                                                      json:"updated_at"`
}

func (c *CustomFieldDefinition) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

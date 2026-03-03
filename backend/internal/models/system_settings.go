package models

import "time"

// SystemSetting is a generic key-value row for system-wide configuration.
// The value column stores arbitrary JSON (string, number, object, or null).
type SystemSetting struct {
	Key       string    `gorm:"primaryKey"      json:"key"`
	Value     string    `gorm:"type:jsonb"      json:"value"`
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime" json:"updated_at"`
}

// TableName specifies the database table name.
func (SystemSetting) TableName() string { return "system_settings" }

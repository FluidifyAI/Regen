package repository

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	keyGlobalFallbackPolicy = "escalation.global_fallback_policy_id"
	KeyInstanceName         = "instance.name"
	KeyTimezone             = "instance.timezone"
	KeyOpenAIAPIKey         = "ai.openai_api_key"
)

// SystemSettingsRepository manages system-wide configuration stored in the
// system_settings key-value table.
type SystemSettingsRepository interface {
	// GetGlobalFallbackPolicyID returns the configured global escalation fallback
	// policy, or nil if none is set.
	GetGlobalFallbackPolicyID() (*uuid.UUID, error)

	// SetGlobalFallbackPolicyID persists the global fallback policy. Pass nil to
	// clear the setting.
	SetGlobalFallbackPolicyID(id *uuid.UUID) error

	// GetString returns the string value for the given key, or "" if not set.
	GetString(key string) (string, error)

	// SetString persists a string value for the given key.
	SetString(key, value string) error
}

type systemSettingsRepository struct{ db *gorm.DB }

// NewSystemSettingsRepository creates a new SystemSettingsRepository.
func NewSystemSettingsRepository(db *gorm.DB) SystemSettingsRepository {
	return &systemSettingsRepository{db: db}
}

func (r *systemSettingsRepository) GetGlobalFallbackPolicyID() (*uuid.UUID, error) {
	var raw string
	err := r.db.Raw(
		"SELECT value FROM system_settings WHERE key = ?", keyGlobalFallbackPolicy,
	).Scan(&raw).Error
	if err != nil || raw == "null" || raw == "" {
		return nil, nil //nolint:nilerr
	}
	// The DB stores the UUID as a JSON string: "\"uuid-here\""
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *systemSettingsRepository) SetGlobalFallbackPolicyID(id *uuid.UUID) error {
	var val string
	if id == nil {
		val = "null"
	} else {
		b, _ := json.Marshal(id.String())
		val = string(b)
	}
	return r.db.Exec(
		"INSERT INTO system_settings (key, value, updated_at) VALUES (?, ?::jsonb, NOW()) "+
			"ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()",
		keyGlobalFallbackPolicy, val,
	).Error
}
func (r *systemSettingsRepository) GetString(key string) (string, error) {
	var raw string
	err := r.db.Raw("SELECT value FROM system_settings WHERE key = ?", key).Scan(&raw).Error
	if err != nil || raw == "null" || raw == "" {
		return "", nil //nolint:nilerr
	}
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return raw, nil // stored without JSON encoding — return as-is
	}
	return s, nil
}

func (r *systemSettingsRepository) SetString(key, value string) error {
	b, _ := json.Marshal(value)
	return r.db.Exec(
		"INSERT INTO system_settings (key, value, updated_at) VALUES (?, ?::jsonb, NOW()) "+
			"ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()",
		key, string(b),
	).Error
}

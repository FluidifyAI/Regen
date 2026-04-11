package repository

import (
	"fmt"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GroupingRuleRepository defines operations for managing grouping rules
type GroupingRuleRepository interface {
	// Create creates a new grouping rule
	Create(rule *models.GroupingRule) error

	// GetByID retrieves a grouping rule by its ID
	GetByID(id uuid.UUID) (*models.GroupingRule, error)

	// GetAll retrieves all grouping rules ordered by priority (ascending)
	GetAll() ([]models.GroupingRule, error)

	// GetEnabled retrieves only enabled grouping rules ordered by priority
	// This is the most common query - used by the grouping engine
	GetEnabled() ([]models.GroupingRule, error)

	// Update updates an existing grouping rule
	Update(rule *models.GroupingRule) error

	// Delete soft-deletes a grouping rule by ID
	Delete(id uuid.UUID) error

	// CheckPriorityConflict checks if another rule already uses this priority
	// Returns the conflicting rule if one exists, nil otherwise
	CheckPriorityConflict(priority int, excludeID uuid.UUID) (*models.GroupingRule, error)
}

// groupingRuleRepository implements GroupingRuleRepository
type groupingRuleRepository struct {
	db *gorm.DB
}

// NewGroupingRuleRepository creates a new grouping rule repository
func NewGroupingRuleRepository(db *gorm.DB) GroupingRuleRepository {
	return &groupingRuleRepository{db: db}
}

// Create creates a new grouping rule
func (r *groupingRuleRepository) Create(rule *models.GroupingRule) error {
	// Validate rule before creation
	if err := validateGroupingRule(rule); err != nil {
		return err
	}

	// Check for priority conflicts
	conflict, err := r.CheckPriorityConflict(rule.Priority, uuid.Nil)
	if err != nil {
		return fmt.Errorf("failed to check priority conflict: %w", err)
	}
	if conflict != nil {
		return fmt.Errorf("priority %d is already used by rule %q", rule.Priority, conflict.Name)
	}

	if err := r.db.Create(rule).Error; err != nil {
		return fmt.Errorf("failed to create grouping rule: %w", err)
	}

	return nil
}

// GetByID retrieves a grouping rule by its ID
func (r *groupingRuleRepository) GetByID(id uuid.UUID) (*models.GroupingRule, error) {
	var rule models.GroupingRule
	err := r.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &NotFoundError{
				Resource: "grouping_rule",
				ID:       id.String(),
			}
		}
		return nil, fmt.Errorf("failed to get grouping rule: %w", err)
	}

	return &rule, nil
}

// GetAll retrieves all grouping rules ordered by priority (ascending)
func (r *groupingRuleRepository) GetAll() ([]models.GroupingRule, error) {
	var rules []models.GroupingRule
	err := r.db.Order("priority ASC").Find(&rules).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all grouping rules: %w", err)
	}

	return rules, nil
}

// GetEnabled retrieves only enabled grouping rules ordered by priority
//
// This is the most frequently called method - used by the grouping engine
// on every alert processing. The query is optimized with an index on (enabled, priority).
func (r *groupingRuleRepository) GetEnabled() ([]models.GroupingRule, error) {
	var rules []models.GroupingRule
	err := r.db.
		Where("enabled = ?", true).
		Order("priority ASC").
		Find(&rules).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get enabled grouping rules: %w", err)
	}

	return rules, nil
}

// Update updates an existing grouping rule
func (r *groupingRuleRepository) Update(rule *models.GroupingRule) error {
	// Validate rule before update
	if err := validateGroupingRule(rule); err != nil {
		return err
	}

	// Check if rule exists
	existing, err := r.GetByID(rule.ID)
	if err != nil {
		return err
	}

	// Check for priority conflicts (excluding this rule)
	if rule.Priority != existing.Priority {
		conflict, err := r.CheckPriorityConflict(rule.Priority, rule.ID)
		if err != nil {
			return fmt.Errorf("failed to check priority conflict: %w", err)
		}
		if conflict != nil {
			return fmt.Errorf("priority %d is already used by rule %q", rule.Priority, conflict.Name)
		}
	}

	// Update all fields except ID and CreatedAt
	err = r.db.Model(rule).
		Select("name", "description", "enabled", "priority", "match_labels", "time_window_seconds", "cross_source_labels", "updated_at").
		Updates(rule).Error

	if err != nil {
		return fmt.Errorf("failed to update grouping rule: %w", err)
	}

	return nil
}

// Delete soft-deletes a grouping rule by ID
func (r *groupingRuleRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&models.GroupingRule{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete grouping rule: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return &NotFoundError{
			Resource: "grouping_rule",
			ID:       id.String(),
		}
	}

	return nil
}

// CheckPriorityConflict checks if another rule already uses this priority
func (r *groupingRuleRepository) CheckPriorityConflict(priority int, excludeID uuid.UUID) (*models.GroupingRule, error) {
	var rule models.GroupingRule
	query := r.db.Where("priority = ?", priority)

	// Exclude a specific rule ID (used during updates)
	if excludeID != uuid.Nil {
		query = query.Where("id != ?", excludeID)
	}

	err := query.First(&rule).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No conflict
		}
		return nil, fmt.Errorf("failed to check priority conflict: %w", err)
	}

	return &rule, nil
}

// validateGroupingRule validates a grouping rule before database operations
func validateGroupingRule(rule *models.GroupingRule) error {
	if rule.Name == "" {
		return fmt.Errorf("grouping rule name cannot be empty")
	}

	if rule.Priority < 0 {
		return fmt.Errorf("grouping rule priority must be non-negative, got %d", rule.Priority)
	}

	if rule.TimeWindowSeconds <= 0 {
		return fmt.Errorf("grouping rule time_window_seconds must be positive, got %d", rule.TimeWindowSeconds)
	}

	// Validate MatchLabels is valid JSONB (will be checked by GORM/PostgreSQL)
	// No additional validation needed here

	return nil
}

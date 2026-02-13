package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
)

// RoutingRuleRepository defines operations for managing routing rules
type RoutingRuleRepository interface {
	// Create creates a new routing rule
	Create(rule *models.RoutingRule) error

	// GetByID retrieves a routing rule by its ID
	GetByID(id uuid.UUID) (*models.RoutingRule, error)

	// GetAll retrieves all routing rules ordered by priority (ascending)
	GetAll() ([]models.RoutingRule, error)

	// GetEnabled retrieves only enabled routing rules ordered by priority.
	// This is the hot-path query — called by the routing engine on every alert.
	GetEnabled() ([]models.RoutingRule, error)

	// Update updates an existing routing rule
	Update(rule *models.RoutingRule) error

	// Delete deletes a routing rule by ID
	Delete(id uuid.UUID) error

	// CheckPriorityConflict checks if another rule already uses this priority.
	// Returns the conflicting rule if one exists, nil otherwise.
	CheckPriorityConflict(priority int, excludeID uuid.UUID) (*models.RoutingRule, error)
}

// routingRuleRepository implements RoutingRuleRepository
type routingRuleRepository struct {
	db *gorm.DB
}

// NewRoutingRuleRepository creates a new routing rule repository
func NewRoutingRuleRepository(db *gorm.DB) RoutingRuleRepository {
	return &routingRuleRepository{db: db}
}

// Create creates a new routing rule
func (r *routingRuleRepository) Create(rule *models.RoutingRule) error {
	if err := validateRoutingRule(rule); err != nil {
		return err
	}

	conflict, err := r.CheckPriorityConflict(rule.Priority, uuid.Nil)
	if err != nil {
		return fmt.Errorf("failed to check priority conflict: %w", err)
	}
	if conflict != nil {
		return fmt.Errorf("priority %d is already used by rule %q", rule.Priority, conflict.Name)
	}

	if err := r.db.Create(rule).Error; err != nil {
		return fmt.Errorf("failed to create routing rule: %w", err)
	}

	return nil
}

// GetByID retrieves a routing rule by its ID
func (r *routingRuleRepository) GetByID(id uuid.UUID) (*models.RoutingRule, error) {
	var rule models.RoutingRule
	err := r.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &NotFoundError{Resource: "routing_rule", ID: id.String()}
		}
		return nil, fmt.Errorf("failed to get routing rule: %w", err)
	}

	return &rule, nil
}

// GetAll retrieves all routing rules ordered by priority (ascending)
func (r *routingRuleRepository) GetAll() ([]models.RoutingRule, error) {
	var rules []models.RoutingRule
	if err := r.db.Order("priority ASC").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("failed to get all routing rules: %w", err)
	}

	return rules, nil
}

// GetEnabled retrieves only enabled routing rules ordered by priority.
// Uses the partial index idx_routing_rules_enabled_priority for performance.
func (r *routingRuleRepository) GetEnabled() ([]models.RoutingRule, error) {
	var rules []models.RoutingRule
	err := r.db.
		Where("enabled = ?", true).
		Order("priority ASC").
		Find(&rules).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get enabled routing rules: %w", err)
	}

	return rules, nil
}

// Update updates an existing routing rule
func (r *routingRuleRepository) Update(rule *models.RoutingRule) error {
	if err := validateRoutingRule(rule); err != nil {
		return err
	}

	existing, err := r.GetByID(rule.ID)
	if err != nil {
		return err
	}

	if rule.Priority != existing.Priority {
		conflict, err := r.CheckPriorityConflict(rule.Priority, rule.ID)
		if err != nil {
			return fmt.Errorf("failed to check priority conflict: %w", err)
		}
		if conflict != nil {
			return fmt.Errorf("priority %d is already used by rule %q", rule.Priority, conflict.Name)
		}
	}

	err = r.db.Model(rule).
		Select("name", "description", "enabled", "priority", "match_criteria", "actions", "updated_at").
		Updates(rule).Error

	if err != nil {
		return fmt.Errorf("failed to update routing rule: %w", err)
	}

	return nil
}

// Delete deletes a routing rule by ID
func (r *routingRuleRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&models.RoutingRule{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete routing rule: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "routing_rule", ID: id.String()}
	}

	return nil
}

// CheckPriorityConflict checks if another rule already uses this priority
func (r *routingRuleRepository) CheckPriorityConflict(priority int, excludeID uuid.UUID) (*models.RoutingRule, error) {
	var rule models.RoutingRule
	query := r.db.Where("priority = ?", priority)

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

// validateRoutingRule validates a routing rule before database operations
func validateRoutingRule(rule *models.RoutingRule) error {
	if rule.Name == "" {
		return fmt.Errorf("routing rule name cannot be empty")
	}

	if rule.Priority < 0 {
		return fmt.Errorf("routing rule priority must be non-negative, got %d", rule.Priority)
	}

	return nil
}

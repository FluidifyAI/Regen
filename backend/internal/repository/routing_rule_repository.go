package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
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

	// Reorder reassigns priorities in bulk using the supplied ordered slice of IDs.
	// Priorities become 10, 20, 30 … (multiples of 10) to leave room for future inserts.
	// Runs inside a single transaction. IDs not present in the slice are unchanged.
	Reorder(ids []uuid.UUID) error

	// GetMaxPriority returns the highest priority value currently in use (0 if no rules exist).
	GetMaxPriority() (int, error)
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

// Reorder reassigns priorities in bulk.
func (r *routingRuleRepository) Reorder(ids []uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			priority := (i + 1) * 10
			if err := tx.Model(&models.RoutingRule{}).Where("id = ?", id).Update("priority", priority).Error; err != nil {
				return fmt.Errorf("failed to update priority for rule %s: %w", id, err)
			}
		}
		return nil
	})
}

// GetMaxPriority returns the highest priority value in use (0 if none).
func (r *routingRuleRepository) GetMaxPriority() (int, error) {
	var max int
	err := r.db.Model(&models.RoutingRule{}).Select("COALESCE(MAX(priority), 0)").Scan(&max).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get max priority: %w", err)
	}
	return max, nil
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

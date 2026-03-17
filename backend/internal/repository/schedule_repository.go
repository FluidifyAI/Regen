package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
	"gorm.io/gorm"
)

// ScheduleRepository defines all database operations for schedules.
type ScheduleRepository interface {
	// --- Schedule CRUD ---

	// Create persists a new schedule. Layers/participants must be created separately.
	Create(schedule *models.Schedule) error

	// GetByID returns the schedule row only (no layers). Use GetWithLayers for full data.
	GetByID(id uuid.UUID) (*models.Schedule, error)

	// GetAll returns all schedules ordered by name, without layers.
	GetAll() ([]models.Schedule, error)

	// Update saves name, description, timezone, notification_channel, and updated_at.
	Update(schedule *models.Schedule) error

	// Delete removes a schedule and cascades to layers, participants, overrides.
	Delete(id uuid.UUID) error

	// GetWithLayers returns the schedule plus all layers (ordered by order_index)
	// plus all participants per layer (ordered by order_index).
	// Executes 3 queries: schedule, layers, participants.
	GetWithLayers(id uuid.UUID) (*models.Schedule, error)

	// --- Layer operations ---

	// CreateLayer adds a new layer to a schedule.
	CreateLayer(layer *models.ScheduleLayer) error

	// DeleteLayer removes a layer (cascades to participants).
	DeleteLayer(layerID uuid.UUID) error

	// UpdateLayer updates layer metadata and optionally replaces participants atomically.
	// A nil participants pointer means "leave participants untouched".
	// A non-nil pointer (even pointing to an empty slice) replaces all participants.
	UpdateLayer(layer *models.ScheduleLayer, participants *[]models.ScheduleParticipant) error

	// CreateParticipantsBulk bulk-inserts participants for a layer.
	CreateParticipantsBulk(participants []models.ScheduleParticipant) error

	// --- Override operations ---

	// CreateOverride adds an override to a schedule.
	CreateOverride(override *models.ScheduleOverride) error

	// DeleteOverride removes an override by ID.
	DeleteOverride(overrideID uuid.UUID) error

	// GetActiveOverrides returns all overrides for a schedule that cover `at`.
	// "Cover" means start_time <= at < end_time.
	// Results are ordered by start_time DESC so the most-recently-starting override is first.
	GetActiveOverrides(scheduleID uuid.UUID, at time.Time) ([]models.ScheduleOverride, error)

	// GetOverridesInWindow returns all overrides for a schedule that overlap [from, to).
	// Used by GetTimeline to collect all overrides in the requested window.
	GetOverridesInWindow(scheduleID uuid.UUID, from, to time.Time) ([]models.ScheduleOverride, error)

	// ListUpcomingOverrides returns all overrides for a schedule whose end_time is in the future,
	// ordered by start_time ASC. Used by the UI override management table.
	ListUpcomingOverrides(scheduleID uuid.UUID) ([]models.ScheduleOverride, error)
}

// scheduleRepository implements ScheduleRepository.
type scheduleRepository struct {
	db *gorm.DB
}

// NewScheduleRepository creates a new schedule repository.
func NewScheduleRepository(db *gorm.DB) ScheduleRepository {
	return &scheduleRepository{db: db}
}

func (r *scheduleRepository) Create(schedule *models.Schedule) error {
	if err := validateSchedule(schedule); err != nil {
		return err
	}
	if err := r.db.Create(schedule).Error; err != nil {
		return fmt.Errorf("failed to create schedule: %w", err)
	}
	return nil
}

func (r *scheduleRepository) GetByID(id uuid.UUID) (*models.Schedule, error) {
	var s models.Schedule
	err := r.db.Where("id = ?", id).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &NotFoundError{Resource: "schedule", ID: id.String()}
		}
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}
	return &s, nil
}

func (r *scheduleRepository) GetAll() ([]models.Schedule, error) {
	var schedules []models.Schedule
	if err := r.db.Order("name ASC").Find(&schedules).Error; err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}
	return schedules, nil
}

func (r *scheduleRepository) Update(schedule *models.Schedule) error {
	if err := validateSchedule(schedule); err != nil {
		return err
	}
	err := r.db.Model(schedule).
		Select("name", "description", "timezone", "notification_channel", "updated_at").
		Updates(schedule).Error
	if err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}
	return nil
}

func (r *scheduleRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&models.Schedule{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete schedule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "schedule", ID: id.String()}
	}
	return nil
}

func (r *scheduleRepository) GetWithLayers(id uuid.UUID) (*models.Schedule, error) {
	// Query 1: schedule
	schedule, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Query 2: layers ordered by order_index
	var layers []models.ScheduleLayer
	if err := r.db.
		Where("schedule_id = ?", id).
		Order("order_index ASC").
		Find(&layers).Error; err != nil {
		return nil, fmt.Errorf("failed to get layers for schedule %s: %w", id, err)
	}

	if len(layers) == 0 {
		schedule.Layers = []models.ScheduleLayer{}
		return schedule, nil
	}

	// Query 3: all participants for these layers in one query
	layerIDs := make([]uuid.UUID, len(layers))
	for i, l := range layers {
		layerIDs[i] = l.ID
	}

	var participants []models.ScheduleParticipant
	if err := r.db.
		Where("layer_id IN ?", layerIDs).
		Order("layer_id, order_index ASC").
		Find(&participants).Error; err != nil {
		return nil, fmt.Errorf("failed to get participants for schedule %s: %w", id, err)
	}

	// Group participants by layer_id
	participantsByLayer := make(map[uuid.UUID][]models.ScheduleParticipant, len(layers))
	for _, p := range participants {
		participantsByLayer[p.LayerID] = append(participantsByLayer[p.LayerID], p)
	}
	for i := range layers {
		layers[i].Participants = participantsByLayer[layers[i].ID]
		if layers[i].Participants == nil {
			layers[i].Participants = []models.ScheduleParticipant{}
		}
	}

	schedule.Layers = layers
	return schedule, nil
}

func (r *scheduleRepository) CreateLayer(layer *models.ScheduleLayer) error {
	if err := validateLayer(layer); err != nil {
		return err
	}
	if err := r.db.Create(layer).Error; err != nil {
		return fmt.Errorf("failed to create schedule layer: %w", err)
	}
	return nil
}

func (r *scheduleRepository) DeleteLayer(layerID uuid.UUID) error {
	result := r.db.Delete(&models.ScheduleLayer{}, "id = ?", layerID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete schedule layer: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "schedule_layer", ID: layerID.String()}
	}
	return nil
}

func (r *scheduleRepository) UpdateLayer(layer *models.ScheduleLayer, participants *[]models.ScheduleParticipant) error {
	if err := validateLayer(layer); err != nil {
		return err
	}
	return r.db.Transaction(func(db *gorm.DB) error {
		if err := db.Model(layer).Updates(map[string]interface{}{
			"name":                   layer.Name,
			"rotation_type":          layer.RotationType,
			"shift_duration_seconds": layer.ShiftDurationSeconds,
			"rotation_start":         layer.RotationStart,
		}).Error; err != nil {
			return fmt.Errorf("failed to update schedule layer: %w", err)
		}
		// Only replace participants if explicitly requested (non-nil pointer).
		if participants != nil {
			if err := db.Where("layer_id = ?", layer.ID).Delete(&models.ScheduleParticipant{}).Error; err != nil {
				return fmt.Errorf("failed to delete existing participants: %w", err)
			}
			if len(*participants) > 0 {
				if err := db.Create(participants).Error; err != nil {
					return fmt.Errorf("failed to bulk create participants: %w", err)
				}
			}
		}
		return nil
	})
}

func (r *scheduleRepository) CreateParticipantsBulk(participants []models.ScheduleParticipant) error {
	if len(participants) == 0 {
		return nil
	}
	if err := r.db.Create(&participants).Error; err != nil {
		return fmt.Errorf("failed to bulk create participants: %w", err)
	}
	return nil
}

func (r *scheduleRepository) CreateOverride(override *models.ScheduleOverride) error {
	if err := validateOverride(override); err != nil {
		return err
	}
	if err := r.db.Create(override).Error; err != nil {
		return fmt.Errorf("failed to create schedule override: %w", err)
	}
	return nil
}

func (r *scheduleRepository) DeleteOverride(overrideID uuid.UUID) error {
	result := r.db.Delete(&models.ScheduleOverride{}, "id = ?", overrideID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete schedule override: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "schedule_override", ID: overrideID.String()}
	}
	return nil
}

func (r *scheduleRepository) GetActiveOverrides(scheduleID uuid.UUID, at time.Time) ([]models.ScheduleOverride, error) {
	var overrides []models.ScheduleOverride
	err := r.db.
		Where("schedule_id = ? AND start_time <= ? AND end_time > ?", scheduleID, at, at).
		Order("start_time DESC").
		Find(&overrides).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active overrides: %w", err)
	}
	return overrides, nil
}

func (r *scheduleRepository) GetOverridesInWindow(scheduleID uuid.UUID, from, to time.Time) ([]models.ScheduleOverride, error) {
	var overrides []models.ScheduleOverride
	// Overlap condition: override starts before window ends AND override ends after window starts
	err := r.db.
		Where("schedule_id = ? AND start_time < ? AND end_time > ?", scheduleID, to, from).
		Order("start_time ASC").
		Find(&overrides).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get overrides in window: %w", err)
	}
	return overrides, nil
}

func (r *scheduleRepository) ListUpcomingOverrides(scheduleID uuid.UUID) ([]models.ScheduleOverride, error) {
	var overrides []models.ScheduleOverride
	err := r.db.
		Where("schedule_id = ? AND end_time > NOW()", scheduleID).
		Order("start_time ASC").
		Find(&overrides).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list upcoming overrides: %w", err)
	}
	return overrides, nil
}

// --- Validation helpers ---

func validateSchedule(s *models.Schedule) error {
	if s.Name == "" {
		return fmt.Errorf("schedule name cannot be empty")
	}
	if s.Timezone == "" {
		return fmt.Errorf("schedule timezone cannot be empty")
	}
	return nil
}

func validateLayer(l *models.ScheduleLayer) error {
	if l.ScheduleID == uuid.Nil {
		return fmt.Errorf("layer must belong to a schedule")
	}
	if l.Name == "" {
		return fmt.Errorf("layer name cannot be empty")
	}
	if l.ShiftDurationSeconds <= 0 {
		return fmt.Errorf("shift_duration_seconds must be positive, got %d", l.ShiftDurationSeconds)
	}
	switch l.RotationType {
	case models.RotationTypeDaily, models.RotationTypeWeekly, models.RotationTypeCustom:
	default:
		return fmt.Errorf("invalid rotation_type %q: must be daily, weekly, or custom", l.RotationType)
	}
	return nil
}

func validateOverride(o *models.ScheduleOverride) error {
	if o.ScheduleID == uuid.Nil {
		return fmt.Errorf("override must belong to a schedule")
	}
	if o.OverrideUser == "" {
		return fmt.Errorf("override_user cannot be empty")
	}
	if !o.EndTime.After(o.StartTime) {
		return fmt.Errorf("override end_time must be after start_time")
	}
	return nil
}

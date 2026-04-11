package services

import (
	"context"
	"fmt"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
)

// PostMortemService manages post-mortem templates, documents, and action items.
type PostMortemService interface {
	// Templates
	ListTemplates() ([]models.PostMortemTemplate, error)
	GetTemplate(id uuid.UUID) (*models.PostMortemTemplate, error)
	CreateTemplate(params *CreatePostMortemTemplateParams) (*models.PostMortemTemplate, error)
	UpdateTemplate(id uuid.UUID, params *UpdatePostMortemTemplateParams) (*models.PostMortemTemplate, error)
	DeleteTemplate(id uuid.UUID) error

	// Post-mortems
	GetPostMortem(incidentID uuid.UUID) (*models.PostMortem, error)
	GeneratePostMortem(incident *models.Incident, templateID *uuid.UUID, createdByID string) (*models.PostMortem, error)
	UpdatePostMortem(id uuid.UUID, params *UpdatePostMortemParams) (*models.PostMortem, error)

	// Post-mortem manual creation
	CreatePostMortem(incidentID uuid.UUID, createdByID string) (*models.PostMortem, error)

	// AI enhance
	EnhancePostMortem(pm *models.PostMortem, content string) (*models.PostMortem, error)

	// Comments
	ListComments(postMortemID uuid.UUID) ([]models.PostMortemComment, error)
	CreateComment(postMortemID uuid.UUID, params *CreateCommentParams) (*models.PostMortemComment, error)
	DeleteComment(id uuid.UUID) error

	// Action items
	CreateActionItem(postMortemID uuid.UUID, params *CreateActionItemParams) (*models.ActionItem, error)
	UpdateActionItem(id uuid.UUID, params *UpdateActionItemParams) (*models.ActionItem, error)
	DeleteActionItem(id uuid.UUID) error
}

// ─── Params ───────────────────────────────────────────────────────────────────

type CreatePostMortemTemplateParams struct {
	Name        string
	Description string
	Sections    []string
}

type UpdatePostMortemTemplateParams struct {
	Name        *string
	Description *string
	Sections    []string
}

type UpdatePostMortemParams struct {
	Content *string
	Status  *models.PostMortemStatus
}

type CreateActionItemParams struct {
	Title   string
	Owner   *string
	DueDate *time.Time
}

type UpdateActionItemParams struct {
	Title   *string
	Owner   *string
	DueDate *time.Time
	Status  *models.ActionItemStatus
}

type CreateCommentParams struct {
	AuthorID   string
	AuthorName string
	Content    string
}

// BuiltInTemplateError is returned when trying to modify or delete a built-in template.
type BuiltInTemplateError struct{ Name string }

func (e *BuiltInTemplateError) Error() string {
	return fmt.Sprintf("cannot delete built-in template %q", e.Name)
}

// ─── Implementation ───────────────────────────────────────────────────────────

type postMortemService struct {
	pmRepo       repository.PostMortemRepository
	templateRepo repository.PostMortemTemplateRepository
	commentRepo  repository.PostMortemCommentRepository
	incidentSvc  IncidentService
	aiSvc        AIService
}

func NewPostMortemService(
	pmRepo repository.PostMortemRepository,
	templateRepo repository.PostMortemTemplateRepository,
	commentRepo repository.PostMortemCommentRepository,
	incidentSvc IncidentService,
	aiSvc AIService,
) PostMortemService {
	return &postMortemService{
		pmRepo:       pmRepo,
		templateRepo: templateRepo,
		commentRepo:  commentRepo,
		incidentSvc:  incidentSvc,
		aiSvc:        aiSvc,
	}
}

// ─── Templates ────────────────────────────────────────────────────────────────

func (s *postMortemService) ListTemplates() ([]models.PostMortemTemplate, error) {
	return s.templateRepo.List()
}

func (s *postMortemService) GetTemplate(id uuid.UUID) (*models.PostMortemTemplate, error) {
	return s.templateRepo.GetByID(id)
}

func (s *postMortemService) CreateTemplate(params *CreatePostMortemTemplateParams) (*models.PostMortemTemplate, error) {
	tmpl := &models.PostMortemTemplate{
		Name:        params.Name,
		Description: params.Description,
		Sections:    models.JSONBArray(params.Sections),
		IsBuiltIn:   false,
	}
	if err := s.templateRepo.Create(tmpl); err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (s *postMortemService) UpdateTemplate(id uuid.UUID, params *UpdatePostMortemTemplateParams) (*models.PostMortemTemplate, error) {
	tmpl, err := s.templateRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if params.Name != nil {
		tmpl.Name = *params.Name
	}
	if params.Description != nil {
		tmpl.Description = *params.Description
	}
	if params.Sections != nil {
		tmpl.Sections = models.JSONBArray(params.Sections)
	}
	if err := s.templateRepo.Update(tmpl); err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (s *postMortemService) DeleteTemplate(id uuid.UUID) error {
	tmpl, err := s.templateRepo.GetByID(id)
	if err != nil {
		return err
	}
	if tmpl.IsBuiltIn {
		return &BuiltInTemplateError{Name: tmpl.Name}
	}
	return s.templateRepo.Delete(id)
}

// ─── Post-Mortems ─────────────────────────────────────────────────────────────

func (s *postMortemService) GetPostMortem(incidentID uuid.UUID) (*models.PostMortem, error) {
	return s.pmRepo.GetByIncidentID(incidentID)
}

// GeneratePostMortem calls the AI to produce a post-mortem draft, then persists it.
// If a post-mortem already exists for the incident it is overwritten (regeneration).
// The incident's timeline entry is written after successful creation/update.
func (s *postMortemService) GeneratePostMortem(incident *models.Incident, templateID *uuid.UUID, createdByID string) (*models.PostMortem, error) {
	if !s.aiSvc.IsEnabled() {
		return nil, fmt.Errorf("AI features are not configured: set OPENAI_API_KEY to enable")
	}

	// Resolve template — fall back to first available if none specified
	tmpl, err := s.resolveTemplate(templateID)
	if err != nil {
		return nil, err
	}

	// Gather context (timeline capped at 200 entries to stay within token budget)
	timeline, _, _ := s.incidentSvc.GetIncidentTimeline(incident.ID, repository.Pagination{Page: 1, PageSize: 200})
	alerts, _ := s.incidentSvc.GetIncidentAlerts(incident.ID)

	content, err := s.aiSvc.GeneratePostMortem(
		context.Background(), incident, timeline, alerts, []string(tmpl.Sections),
	)
	if err != nil {
		return nil, fmt.Errorf("generate post-mortem content: %w", err)
	}

	now := time.Now().UTC()

	// Upsert: INSERT ... ON CONFLICT (incident_id) DO UPDATE — safe for concurrent callers.
	// PublishedAt is intentionally nil to reset regenerated drafts to unpublished.
	pm := &models.PostMortem{
		IncidentID:   incident.ID,
		TemplateID:   templateID,
		TemplateName: tmpl.Name,
		Status:       models.PostMortemStatusDraft,
		Content:      content,
		GeneratedBy:  "ai",
		GeneratedAt:  &now,
		CreatedByID:  createdByID,
	}
	if err := s.pmRepo.Upsert(pm); err != nil {
		return nil, err
	}
	// Re-fetch to get the canonical row (with correct ID and action items) after upsert.
	pm, err = s.pmRepo.GetByIncidentID(incident.ID)
	if err != nil {
		return nil, err
	}

	// Record timeline entry for both creation and regeneration (non-fatal)
	_, _ = s.incidentSvc.CreateTimelineEntry(&CreateTimelineEntryParams{
		IncidentID: incident.ID,
		Type:       models.TimelineTypePostmortemCreated,
		ActorType:  "system",
		ActorID:    "ai",
		Content:    models.JSONB{"template": tmpl.Name},
	})

	return pm, nil
}

func (s *postMortemService) UpdatePostMortem(id uuid.UUID, params *UpdatePostMortemParams) (*models.PostMortem, error) {
	pm, err := s.pmRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if params.Content != nil {
		pm.Content = *params.Content
	}
	if params.Status != nil {
		pm.Status = *params.Status
		if *params.Status == models.PostMortemStatusPublished && pm.PublishedAt == nil {
			now := time.Now().UTC()
			pm.PublishedAt = &now
		} else if *params.Status == models.PostMortemStatusDraft {
			pm.PublishedAt = nil
		}
	}
	if err := s.pmRepo.Update(pm); err != nil {
		return nil, err
	}
	return s.pmRepo.GetByID(pm.ID)
}

// ─── Action Items ─────────────────────────────────────────────────────────────

func (s *postMortemService) CreateActionItem(postMortemID uuid.UUID, params *CreateActionItemParams) (*models.ActionItem, error) {
	item := &models.ActionItem{
		PostMortemID: postMortemID,
		Title:        params.Title,
		Owner:        params.Owner,
		DueDate:      params.DueDate,
		Status:       models.ActionItemStatusOpen,
	}
	if err := s.pmRepo.CreateActionItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *postMortemService) UpdateActionItem(id uuid.UUID, params *UpdateActionItemParams) (*models.ActionItem, error) {
	item, err := s.pmRepo.GetActionItemByID(id)
	if err != nil {
		return nil, err
	}
	if params.Title != nil {
		item.Title = *params.Title
	}
	if params.Owner != nil {
		item.Owner = params.Owner
	}
	if params.DueDate != nil {
		item.DueDate = params.DueDate
	}
	if params.Status != nil {
		item.Status = *params.Status
	}
	if err := s.pmRepo.UpdateActionItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *postMortemService) DeleteActionItem(id uuid.UUID) error {
	return s.pmRepo.DeleteActionItem(id)
}

// ─── CreatePostMortem / EnhancePostMortem / Comments ─────────────────────────

func (s *postMortemService) CreatePostMortem(incidentID uuid.UUID, createdByID string) (*models.PostMortem, error) {
	now := time.Now().UTC()
	pm := &models.PostMortem{
		IncidentID:   incidentID,
		TemplateName: "Manual",
		Status:       models.PostMortemStatusDraft,
		Content:      "",
		GeneratedBy:  "manual",
		GeneratedAt:  &now,
		CreatedByID:  createdByID,
	}
	if err := s.pmRepo.Create(pm); err != nil {
		return nil, err
	}
	return pm, nil
}

func (s *postMortemService) EnhancePostMortem(pm *models.PostMortem, content string) (*models.PostMortem, error) {
	if !s.aiSvc.IsEnabled() {
		return nil, fmt.Errorf("AI not configured")
	}
	enhanced, err := s.aiSvc.EnhancePostMortem(context.Background(), content)
	if err != nil {
		return nil, fmt.Errorf("AI enhance failed: %w", err)
	}
	now := time.Now().UTC()
	pm.Content = enhanced
	pm.GeneratedBy = "ai"
	pm.GeneratedAt = &now
	pm.Status = models.PostMortemStatusDraft
	if err := s.pmRepo.Update(pm); err != nil {
		return nil, err
	}
	return s.pmRepo.GetByID(pm.ID)
}

func (s *postMortemService) ListComments(postMortemID uuid.UUID) ([]models.PostMortemComment, error) {
	return s.commentRepo.ListByPostMortemID(postMortemID)
}

func (s *postMortemService) CreateComment(postMortemID uuid.UUID, params *CreateCommentParams) (*models.PostMortemComment, error) {
	comment := &models.PostMortemComment{
		PostMortemID: postMortemID,
		AuthorID:     params.AuthorID,
		AuthorName:   params.AuthorName,
		Content:      params.Content,
	}
	if err := s.commentRepo.Create(comment); err != nil {
		return nil, err
	}
	return comment, nil
}

func (s *postMortemService) DeleteComment(id uuid.UUID) error {
	return s.commentRepo.Delete(id)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// resolveTemplate returns the specified template, or the first built-in if nil.
func (s *postMortemService) resolveTemplate(templateID *uuid.UUID) (*models.PostMortemTemplate, error) {
	if templateID != nil {
		return s.templateRepo.GetByID(*templateID)
	}
	templates, err := s.templateRepo.List()
	if err != nil || len(templates) == 0 {
		// No templates seeded yet — use a hardcoded fallback so generation still works
		return &models.PostMortemTemplate{
			Name:     "Standard",
			Sections: models.JSONBArray{"Summary", "Impact", "Timeline", "Root Cause", "Contributing Factors", "Action Items"},
		}, nil
	}
	return &templates[0], nil
}

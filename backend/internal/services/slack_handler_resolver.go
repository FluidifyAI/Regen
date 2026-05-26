package services

import (
	"sync"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
)

// SlackHandlerResolver lazily initializes the SlackEventHandler from DB config.
// Routes are always registered; the handler is created on first successful request
// after Slack is configured — no server restart needed.
type SlackHandlerResolver struct {
	mu          sync.RWMutex
	handler     *SlackEventHandler
	repo        repository.SlackConfigRepository
	incidentSvc IncidentService
	chatService ChatService
	userRepo    repository.UserRepository
	pmRepo      repository.PostMortemRepository
	aiSvc       AIService
}

func NewSlackHandlerResolver(
	repo repository.SlackConfigRepository,
	incidentSvc IncidentService,
	chatService ChatService,
	userRepo repository.UserRepository,
	pmRepo repository.PostMortemRepository,
	aiSvc AIService,
) *SlackHandlerResolver {
	return &SlackHandlerResolver{
		repo:        repo,
		incidentSvc: incidentSvc,
		chatService: chatService,
		userRepo:    userRepo,
		pmRepo:      pmRepo,
		aiSvc:       aiSvc,
	}
}

// Get returns the cached handler, or tries to initialize one from the current DB config.
// Returns nil if Slack is not yet configured or the token is invalid.
func (r *SlackHandlerResolver) Get() *SlackEventHandler {
	r.mu.RLock()
	h := r.handler
	r.mu.RUnlock()
	if h != nil {
		return h
	}
	return r.tryInit()
}

// Invalidate clears the cached handler so the next request re-reads from DB.
// Call this after Slack config is saved or deleted.
func (r *SlackHandlerResolver) Invalidate() {
	r.mu.Lock()
	r.handler = nil
	r.mu.Unlock()
}

func (r *SlackHandlerResolver) tryInit() *SlackEventHandler {
	if r.chatService == nil {
		return nil
	}
	cfg, err := r.repo.Get()
	if err != nil || cfg == nil || cfg.BotToken == "" {
		return nil
	}
	h, err := NewSlackEventHandler(cfg.BotToken, r.incidentSvc, r.chatService, r.userRepo, r.pmRepo)
	if err != nil {
		return nil
	}
	if r.aiSvc != nil {
		h.SetAIService(r.aiSvc)
	}
	r.mu.Lock()
	r.handler = h
	r.mu.Unlock()
	return h
}

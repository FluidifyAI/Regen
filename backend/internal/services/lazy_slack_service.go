package services

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
)

// ErrSlackNotConfigured is returned by LazySlackService when no valid Slack
// config exists in the database. Callers that treat "not configured" as a
// no-op should check for this error with errors.Is.
var ErrSlackNotConfigured = errors.New("slack not configured")

// lazySlackService implements ChatService by reading Slack config from the DB
// on demand. It caches the real slackService and rebuilds it whenever the
// bot_token in the DB changes — so Slack can be configured or updated through
// the UI without restarting the server.
type lazySlackService struct {
	repo  repository.SlackConfigRepository
	mu    sync.RWMutex
	inner ChatService // cached real service; nil when not yet initialized
	token string      // token used to build inner
}

// NewLazySlackService returns a ChatService backed by the given config repo.
// It is always non-nil; individual calls return ErrSlackNotConfigured when
// Slack has not been set up yet.
func NewLazySlackService(repo repository.SlackConfigRepository) ChatService {
	return &lazySlackService{repo: repo}
}

// resolve returns the real slackService, rebuilding it if the token changed.
// Returns (nil, ErrSlackNotConfigured) when Slack is not configured.
func (l *lazySlackService) resolve() (ChatService, error) {
	cfg, err := l.repo.Get()
	if err != nil {
		return nil, fmt.Errorf("slack config: %w", err)
	}
	if cfg == nil || cfg.BotToken == "" {
		return nil, ErrSlackNotConfigured
	}

	// Fast path: token unchanged and service already built.
	l.mu.RLock()
	if l.token == cfg.BotToken && l.inner != nil {
		svc := l.inner
		l.mu.RUnlock()
		return svc, nil
	}
	l.mu.RUnlock()

	// Slow path: need to (re)initialize.
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.token == cfg.BotToken && l.inner != nil {
		return l.inner, nil // another goroutine beat us
	}
	svc, err := NewSlackService(cfg.BotToken)
	if err != nil {
		// Clear cached service so the next call retries.
		l.inner = nil
		l.token = ""
		return nil, fmt.Errorf("slack auth failed: %w", err)
	}
	l.inner = svc
	l.token = cfg.BotToken
	slog.Info("slack service initialized", "workspace", cfg.WorkspaceName)
	return svc, nil
}

func (l *lazySlackService) CreateChannel(name, description string) (*Channel, error) {
	svc, err := l.resolve()
	if err != nil {
		return nil, err
	}
	return svc.CreateChannel(name, description)
}

func (l *lazySlackService) PostMessage(channelID string, message Message) (string, error) {
	svc, err := l.resolve()
	if err != nil {
		return "", err
	}
	return svc.PostMessage(channelID, message)
}

func (l *lazySlackService) UpdateMessage(channelID, messageTS string, message Message) error {
	svc, err := l.resolve()
	if err != nil {
		return err
	}
	return svc.UpdateMessage(channelID, messageTS, message)
}

func (l *lazySlackService) ArchiveChannel(channelID string) error {
	svc, err := l.resolve()
	if err != nil {
		return err
	}
	return svc.ArchiveChannel(channelID)
}

func (l *lazySlackService) InviteUsers(channelID string, userIDs []string) error {
	svc, err := l.resolve()
	if err != nil {
		return err
	}
	return svc.InviteUsers(channelID, userIDs)
}

func (l *lazySlackService) SendDirectMessage(username string, message Message) error {
	svc, err := l.resolve()
	if err != nil {
		return err
	}
	return svc.SendDirectMessage(username, message)
}

func (l *lazySlackService) GetThreadMessages(channelID, threadTS string) ([]string, error) {
	svc, err := l.resolve()
	if err != nil {
		return nil, err
	}
	return svc.GetThreadMessages(channelID, threadTS)
}

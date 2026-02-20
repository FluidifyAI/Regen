package worker

import (
	"context"
	"log/slog"

	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
	"gorm.io/gorm"
)

// StartAll starts all background workers.
// Should be called from main.go with the application lifecycle context.
func StartAll(ctx context.Context, db *gorm.DB, cfg *config.Config) {
	// Initialize dependencies for the shift notifier
	scheduleRepo := repository.NewScheduleRepository(db)
	scheduleEvaluator := services.NewScheduleEvaluator(scheduleRepo)

	// ChatService is optional — if Slack is not configured, the workers run
	// but all DM sends become graceful no-ops.
	var chatService services.ChatService
	if cfg.SlackBotToken != "" {
		var err error
		chatService, err = services.NewSlackService(cfg.SlackBotToken)
		if err != nil {
			slog.Warn("failed to initialize slack for workers (will skip notifications)",
				"error", err)
		}
	}

	// TeamsService is also optional — initialize if configured (v0.8+).
	// When both Slack and Teams are configured, both receive notifications.
	var teamsChatService services.ChatService
	if cfg.TeamsAppID != "" {
		teamsSvc, err := services.NewTeamsService(cfg.TeamsAppID, cfg.TeamsAppPassword, cfg.TeamsTenantID, cfg.TeamsTeamID)
		if err != nil {
			slog.Warn("failed to initialize teams for workers (will skip teams notifications)", "error", err)
		} else {
			teamsChatService = teamsSvc
		}
	}

	// Build a multi-provider chat service that fans out to Slack and/or Teams.
	// The shift notifier and escalation worker use this for on-call DMs.
	activeChatServices := make([]services.ChatService, 0, 2)
	if chatService != nil {
		activeChatServices = append(activeChatServices, chatService)
	}
	if teamsChatService != nil {
		activeChatServices = append(activeChatServices, teamsChatService)
	}
	var workerChatService services.ChatService
	switch len(activeChatServices) {
	case 1:
		workerChatService = activeChatServices[0]
	case 2:
		workerChatService = services.NewMultiChatService(activeChatServices...)
	}

	// Start the shift notifier
	notifier := NewShiftNotifier(scheduleRepo, scheduleEvaluator, workerChatService)
	go notifier.Run(ctx)

	// Start the escalation worker.
	// Build the escalation engine here with the worker as its notifier so that
	// the engine can immediately send DMs when a tier fires.
	escalationPolicyRepo := repository.NewEscalationPolicyRepository(db)
	escalationWorker := NewEscalationWorker(workerChatService)
	escalationEngine := services.NewEscalationEngine(escalationPolicyRepo, scheduleEvaluator, escalationWorker)
	escalationWorker.SetEngine(escalationEngine)
	go escalationWorker.Run(ctx)

	slog.Info("background workers started")
}

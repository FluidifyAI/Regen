package coordinator

import (
	"errors"
	"log/slog"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
)

const PostMortemAgentEmail = "agent-postmortem@system.internal"

// SeedAgents ensures all AI agent user accounts exist in the database.
// Safe to call on every startup — existing agents are left unchanged.
func SeedAgents(userRepo repository.UserRepository) error {
	if err := seedAgent(userRepo, PostMortemAgentEmail, "Post-Mortem Agent", "postmortem"); err != nil {
		return err
	}
	slog.Info("AI agents seeded")
	return nil
}

func seedAgent(userRepo repository.UserRepository, email, name, agentType string) error {
	existing, err := userRepo.GetByEmail(email)
	if err == nil {
		// Agent exists — restore it if it was accidentally deactivated.
		if existing.AuthSource != "ai" {
			if restoreErr := userRepo.RestoreAgent(existing.ID); restoreErr != nil {
				return restoreErr
			}
			slog.Info("restored AI agent", "name", name, "email", email)
		}
		return nil
	}

	// GetByEmail returns *repository.NotFoundError when the user does not exist.
	// Any other error (e.g. DB connection failure) must be propagated.
	var notFound *repository.NotFoundError
	if !errors.As(err, &notFound) {
		return err
	}

	at := agentType
	agent := &models.User{
		Email:      email,
		Name:       name,
		AuthSource: "ai",
		AgentType:  &at,
		Role:       models.UserRoleMember,
		Active:     true,
	}
	if createErr := userRepo.CreateAgent(agent); createErr != nil {
		return createErr
	}
	slog.Info("seeded AI agent", "name", name, "email", email)
	return nil
}

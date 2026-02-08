package services

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/slack-go/slack"
)

// SlackValidator validates Slack configuration and permissions.
type SlackValidator struct {
	client *slack.Client
}

// NewSlackValidator creates a new Slack validator.
func NewSlackValidator(token string) *SlackValidator {
	return &SlackValidator{
		client: slack.New(token),
	}
}

// ValidateToken validates the Slack bot token by calling auth.test.
// This verifies the token is valid and active.
func (v *SlackValidator) ValidateToken() error {
	auth, err := v.client.AuthTest()
	if err != nil {
		return fmt.Errorf("slack token is invalid: %w", err)
	}

	slog.Info("slack token validated",
		"bot_id", auth.BotID,
		"bot_user_id", auth.UserID,
		"team", auth.Team,
		"team_id", auth.TeamID)

	return nil
}

// ValidateScopes validates that the bot has the required OAuth scopes.
// Since auth.test doesn't return scopes directly, we verify by attempting
// operations that require each scope where possible without side effects.
//
// Required scopes:
// - channels:manage - Create and manage channels
// - channels:read - Read channel information
// - chat:write - Post messages
//
// Note: channels:manage and chat:write cannot be validated without side effects
// (creating channels/posting messages). These scopes will be validated on first use,
// and the Slack API will return clear error messages if they are missing.
func (v *SlackValidator) ValidateScopes() error {
	requiredScopes := []string{
		"channels:manage",
		"channels:read",
		"chat:write",
		"users:read",
	}

	slog.Info("validating slack scopes", "required_scopes", requiredScopes)

	// Test channels:read scope
	// Try to list conversations (requires channels:read)
	_, _, err := v.client.GetConversationsForUser(&slack.GetConversationsForUserParameters{
		Limit: 1,
	})
	if err != nil {
		if isPermissionError(err) {
			return fmt.Errorf("missing 'channels:read' scope: bot cannot read channel information. Please add this scope in your Slack app settings at https://api.slack.com/apps")
		}
		// Other errors (network, etc.) are not scope issues
		slog.Warn("could not verify channels:read scope due to API error", "error", err)
	} else {
		slog.Info("channels:read scope validated")
	}

	// Note: channels:manage, chat:write, and users:read scopes cannot be validated without side effects
	// (creating a test channel, posting a test message, or inviting users). These will be validated on first use.
	// If these scopes are missing, users will get clear error messages when attempting to:
	// - Create incident channels (channels:manage required)
	// - Post incident messages (chat:write required)
	// - Invite users to channels (users:read required)
	slog.Warn("channels:manage, chat:write, and users:read scopes cannot be validated without side effects - will be verified on first use",
		"impact", "if these scopes are missing, channel creation, message posting, or user invitation will fail with clear error messages")

	slog.Info("slack scopes validation completed", "validated_scopes", []string{"channels:read"}, "deferred_scopes", []string{"channels:manage", "chat:write", "users:read"})
	return nil
}

// ValidateConfiguration performs complete validation of Slack configuration.
// This is a convenience method that calls both ValidateToken and ValidateScopes.
func (v *SlackValidator) ValidateConfiguration() error {
	if err := v.ValidateToken(); err != nil {
		return err
	}

	if err := v.ValidateScopes(); err != nil {
		return err
	}

	return nil
}

// isPermissionError checks if an error is due to missing OAuth scopes.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "missing_scope") ||
		strings.Contains(errStr, "not_allowed") ||
		strings.Contains(errStr, "invalid_scope")
}

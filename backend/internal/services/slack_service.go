package services

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

// slackService implements the ChatService interface using the Slack API.
type slackService struct {
	client *slack.Client
	teamID string
}

// NewSlackService creates a new Slack implementation of ChatService.
// It validates the token on initialization by calling auth.test.
//
// Parameters:
//   - token: Slack bot token (xoxb-...)
//
// Returns:
//   - ChatService implementation
//   - Error if token is invalid or auth fails
func NewSlackService(token string) (ChatService, error) {
	if token == "" {
		return nil, fmt.Errorf("slack bot token is required")
	}

	client := slack.New(token)

	// Validate token on startup
	auth, err := client.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("slack auth failed: %w", err)
	}

	slog.Info("slack service initialized",
		"bot_id", auth.BotID,
		"user_id", auth.UserID,
		"team", auth.Team,
		"team_id", auth.TeamID)

	return &slackService{
		client: client,
		teamID: auth.TeamID,
	}, nil
}

// CreateChannel creates a new public Slack channel.
// If the channel name already exists, it appends a numeric suffix (-1, -2, etc.).
func (s *slackService) CreateChannel(name, description string) (*Channel, error) {
	// Sanitize channel name to meet Slack requirements
	sanitized := sanitizeChannelName(name)

	if sanitized == "" {
		return nil, fmt.Errorf("channel name is empty after sanitization")
	}

	// Attempt to create channel
	channel, err := s.createChannelWithRetry(sanitized, 0)
	if err != nil {
		return nil, err
	}

	// Set channel topic/description if provided
	if description != "" {
		_, err = s.client.SetTopicOfConversation(channel.ID, description)
		if err != nil {
			slog.Warn("failed to set channel topic",
				"channel_id", channel.ID,
				"error", err)
			// Non-fatal - channel was created successfully
		}
	}

	return &Channel{
		ID:   channel.ID,
		Name: channel.Name,
		URL:  fmt.Sprintf("https://app.slack.com/client/%s/%s", s.teamID, channel.ID),
	}, nil
}

// createChannelWithRetry attempts to create a channel, handling name collisions
// by appending numeric suffixes.
func (s *slackService) createChannelWithRetry(baseName string, attempt int) (*slack.Channel, error) {
	channelName := baseName
	if attempt > 0 {
		channelName = fmt.Sprintf("%s-%d", baseName, attempt)
	}

	// Ensure name doesn't exceed Slack's 80 character limit
	if len(channelName) > 80 {
		// Truncate base, keeping suffix intact
		maxBase := 80 - len(fmt.Sprintf("-%d", attempt))
		if attempt > 0 {
			channelName = fmt.Sprintf("%s-%d", baseName[:maxBase], attempt)
		} else {
			channelName = baseName[:80]
		}
	}

	channel, err := s.client.CreateConversation(slack.CreateConversationParams{
		ChannelName: channelName,
		IsPrivate:   false,
	})

	if err != nil {
		// Handle name collision
		if isNameCollisionError(err) {
			if attempt >= 10 {
				return nil, fmt.Errorf("failed to create channel after 10 attempts: %w", err)
			}
			slog.Debug("channel name collision, retrying with suffix",
				"name", channelName,
				"attempt", attempt+1)
			return s.createChannelWithRetry(baseName, attempt+1)
		}

		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	return channel, nil
}

// PostMessage posts a message to a Slack channel.
func (s *slackService) PostMessage(channelID string, message Message) (string, error) {
	opts := []slack.MsgOption{
		slack.MsgOptionText(message.Text, false),
	}

	// Add blocks if provided
	if len(message.Blocks) > 0 {
		blocks := convertToSlackBlocks(message.Blocks)
		opts = append(opts, slack.MsgOptionBlocks(blocks...))
	}

	// Add thread timestamp if replying in thread
	if message.ThreadTS != "" {
		opts = append(opts, slack.MsgOptionTS(message.ThreadTS))
	}

	// Post message with retry on rate limit
	var timestamp string
	var err error
	for i := 0; i < 3; i++ {
		_, timestamp, err = s.client.PostMessage(channelID, opts...)
		if err == nil {
			return timestamp, nil
		}

		// Check if rate limited
		if isRateLimitError(err) {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(i)) * time.Second
			slog.Warn("rate limited, retrying",
				"attempt", i+1,
				"backoff_seconds", backoff.Seconds())
			time.Sleep(backoff)
			continue
		}

		// Non-rate-limit error, fail immediately
		return "", fmt.Errorf("failed to post message: %w", err)
	}

	return "", fmt.Errorf("failed to post message after retries: %w", err)
}

// UpdateMessage updates an existing message in a Slack channel.
func (s *slackService) UpdateMessage(channelID, messageTS string, message Message) error {
	opts := []slack.MsgOption{
		slack.MsgOptionText(message.Text, false),
	}

	// Add blocks if provided
	if len(message.Blocks) > 0 {
		blocks := convertToSlackBlocks(message.Blocks)
		opts = append(opts, slack.MsgOptionBlocks(blocks...))
	}

	// Update message with retry on rate limit
	for i := 0; i < 3; i++ {
		_, _, _, err := s.client.UpdateMessage(channelID, messageTS, opts...)
		if err == nil {
			return nil
		}

		// Check if rate limited
		if isRateLimitError(err) {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(i)) * time.Second
			slog.Warn("rate limited, retrying",
				"attempt", i+1,
				"backoff_seconds", backoff.Seconds())
			time.Sleep(backoff)
			continue
		}

		// Non-rate-limit error, fail immediately
		return fmt.Errorf("failed to update message: %w", err)
	}

	return fmt.Errorf("failed to update message after retries")
}

// ArchiveChannel archives a Slack channel.
func (s *slackService) ArchiveChannel(channelID string) error {
	err := s.client.ArchiveConversation(channelID)
	if err != nil {
		return fmt.Errorf("failed to archive channel: %w", err)
	}

	return nil
}

// Helper functions

// sanitizeChannelName converts a string into a valid Slack channel name.
// Rules:
// - Lowercase only
// - Replace invalid characters (spaces, underscores) with hyphens
// - Remove consecutive hyphens
// - Trim hyphens from start/end
// - Max 80 characters (Slack limit)
func sanitizeChannelName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	name = reg.ReplaceAllString(name, "-")

	// Replace multiple consecutive hyphens with single hyphen
	reg = regexp.MustCompile(`-+`)
	name = reg.ReplaceAllString(name, "-")

	// Trim hyphens from start and end
	name = strings.Trim(name, "-")

	// Truncate to 80 characters
	if len(name) > 80 {
		name = name[:80]
		name = strings.TrimRight(name, "-")
	}

	return name
}

// formatIncidentChannelName creates a channel name following the convention:
// inc-{number}-{slug}
func formatIncidentChannelName(incidentNumber int, slug string) string {
	base := fmt.Sprintf("inc-%d-%s", incidentNumber, slug)
	return sanitizeChannelName(base)
}

// isNameCollisionError checks if the error is due to a channel name already existing.
func isNameCollisionError(err error) bool {
	if err == nil {
		return false
	}
	// Slack returns "name_taken" error when channel already exists
	return strings.Contains(err.Error(), "name_taken")
}

// isRateLimitError checks if the error is due to rate limiting.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// Slack returns "rate_limited" error or HTTP 429
	errStr := err.Error()
	return strings.Contains(errStr, "rate_limited") ||
		strings.Contains(errStr, "429")
}

// convertToSlackBlocks converts []interface{} to []slack.Block.
// This is needed because the ChatService interface uses []interface{} to remain
// platform-agnostic, but the Slack SDK expects []slack.Block.
func convertToSlackBlocks(blocks []interface{}) []slack.Block {
	result := make([]slack.Block, 0, len(blocks))
	for _, block := range blocks {
		if slackBlock, ok := block.(slack.Block); ok {
			result = append(result, slackBlock)
		}
	}
	return result
}

package services

// ChatService defines the interface for chat platform operations.
// This abstraction enables support for multiple chat platforms (Slack, Teams)
// while keeping business logic platform-agnostic.
//
// Implementations should:
// - Handle authentication internally
// - Implement exponential backoff for rate limits
// - Return descriptive errors for debugging
// - Be safe for concurrent use
type ChatService interface {
	// CreateChannel creates a new channel with the given name and description.
	// The channel should be public by default.
	//
	// Parameters:
	//   - name: The channel name (will be sanitized by implementation)
	//   - description: Optional channel description/topic
	//
	// Returns:
	//   - Channel with ID, Name, and URL populated
	//   - Error if creation fails (auth, network, name collision, etc.)
	//
	// Thread-safe: Yes
	CreateChannel(name, description string) (*Channel, error)

	// PostMessage posts a message to the specified channel.
	//
	// Parameters:
	//   - channelID: The platform-specific channel identifier
	//   - message: The message to post (text + optional blocks)
	//
	// Returns:
	//   - Message timestamp/ID for future updates
	//   - Error if posting fails (invalid channel, permissions, rate limits, network, etc.)
	//
	// Thread-safe: Yes
	PostMessage(channelID string, message Message) (string, error)

	// UpdateMessage updates an existing message in a channel.
	//
	// Parameters:
	//   - channelID: The platform-specific channel identifier
	//   - messageTS: The message timestamp/ID returned from PostMessage
	//   - message: The updated message content
	//
	// Returns:
	//   - Error if update fails (message not found, invalid channel, permissions, rate limits, network, etc.)
	//
	// Thread-safe: Yes
	UpdateMessage(channelID, messageTS string, message Message) error

	// ArchiveChannel archives the specified channel.
	// Archived channels are read-only and hidden from active channel lists.
	//
	// Parameters:
	//   - channelID: The platform-specific channel identifier
	//
	// Returns:
	//   - Error if archiving fails (channel not found, permissions, already archived, network, etc.)
	//
	// Thread-safe: Yes
	ArchiveChannel(channelID string) error
}

// Channel represents a chat channel with platform-specific details.
type Channel struct {
	// ID is the platform-specific channel identifier
	// Examples: "C01234567" (Slack), "19:xxx@thread.tacv2" (Teams)
	ID string

	// Name is the human-readable channel name
	// Examples: "inc-042-api-gateway-errors"
	Name string

	// URL is the direct link to the channel in the platform's web/app UI
	URL string
}

// Message represents a chat message with platform-agnostic content.
// Implementations should convert Blocks to their platform's format.
type Message struct {
	// Text is the plain text fallback content
	// Required for notifications and accessibility
	Text string

	// Blocks contains platform-specific rich content blocks
	// For Slack: []slack.Block
	// For Teams: []teams.MessageBlock
	// Stored as []interface{} to remain platform-agnostic
	Blocks []interface{}

	// ThreadTS is the parent message timestamp for threaded replies
	// Empty string for top-level messages
	ThreadTS string
}

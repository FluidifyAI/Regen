package services

import "fmt"

// multiChatService fans out ChatService calls to multiple providers.
// Used by background workers to send on-call DMs via both Slack and Teams
// when both are configured.
//
// Note: CreateChannel / PostMessage / UpdateMessage / ArchiveChannel are NOT
// forwarded here because channel management is handled per-provider in the
// incident service. Only DM-related operations (SendDirectMessage) are fanned out.
type multiChatService struct {
	providers []ChatService
}

// NewMultiChatService creates a ChatService that fans DMs out to all providers.
func NewMultiChatService(providers ...ChatService) ChatService {
	return &multiChatService{providers: providers}
}

func (m *multiChatService) CreateChannel(name, description string) (*Channel, error) {
	// Multi-provider channel creation is handled in incidentService directly.
	// This path should not be reached in practice.
	return nil, fmt.Errorf("multiChatService.CreateChannel: use per-provider channel creation")
}

func (m *multiChatService) PostMessage(channelID string, message Message) (string, error) {
	// Workers don't post to incident channels through the multi-service.
	return "", fmt.Errorf("multiChatService.PostMessage: use per-provider channel messaging")
}

func (m *multiChatService) UpdateMessage(channelID, messageTS string, message Message) error {
	return fmt.Errorf("multiChatService.UpdateMessage: use per-provider channel messaging")
}

func (m *multiChatService) ArchiveChannel(channelID string) error {
	return fmt.Errorf("multiChatService.ArchiveChannel: use per-provider channel management")
}

func (m *multiChatService) InviteUsers(channelID string, userIDs []string) error {
	return fmt.Errorf("multiChatService.InviteUsers: use per-provider channel management")
}

// SendDirectMessage fans out to all configured providers.
// Best-effort: continues even if one provider fails.
func (m *multiChatService) SendDirectMessage(username string, message Message) error {
	var errs []error
	for _, p := range m.providers {
		if err := p.SendDirectMessage(username, message); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == len(m.providers) && len(errs) > 0 {
		// All providers failed
		return fmt.Errorf("all chat providers failed to send DM to %s: %v", username, errs)
	}
	return nil
}

func (m *multiChatService) GetThreadMessages(channelID, threadTS string) ([]string, error) {
	return nil, fmt.Errorf("multiChatService.GetThreadMessages: use per-provider channel messaging")
}

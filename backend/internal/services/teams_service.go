package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

const graphBaseURL = "https://graph.microsoft.com/v1.0"

// TeamsService implements ChatService using Microsoft Graph API.
// Requires an Azure App Registration with the following application permissions:
//   - ChannelSettings.ReadWrite.All (create/archive channels)
//   - ChannelMessage.Send (post messages)
//   - TeamMember.Read.All (list members for invites)
type TeamsService struct {
	teamID     string
	httpClient *http.Client
}

// NewTeamsService creates a TeamsService authenticated via client credentials (app-only).
// appID, appPassword, tenantID come from an Azure App Registration.
// teamID is the GUID of the Team where incident channels will be created.
func NewTeamsService(appID, appPassword, tenantID, teamID string) (*TeamsService, error) {
	cfg := clientcredentials.Config{
		ClientID:     appID,
		ClientSecret: appPassword,
		TokenURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID),
		Scopes:       []string{"https://graph.microsoft.com/.default"},
	}
	httpClient := cfg.Client(context.Background())
	httpClient.Timeout = 30 * time.Second

	// Validate credentials by fetching team info
	svc := &TeamsService{teamID: teamID, httpClient: httpClient}
	if _, err := svc.getTeam(); err != nil {
		return nil, fmt.Errorf("teams: credential validation failed (check TEAMS_* env vars and app permissions): %w", err)
	}
	return svc, nil
}

// ─── ChatService implementation ───────────────────────────────────────────────
// Methods use context.Background() internally so TeamsService implements the
// context-free ChatService interface while still supporting cancellable HTTP calls.

func (s *TeamsService) CreateChannel(name, description string) (*Channel, error) {
	body := map[string]interface{}{
		"displayName":    name,
		"description":    description,
		"membershipType": "standard",
	}
	var result struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		WebURL      string `json:"webUrl"`
	}
	url := fmt.Sprintf("%s/teams/%s/channels", graphBaseURL, s.teamID)
	if err := s.graphPost(url, body, &result); err != nil {
		return nil, fmt.Errorf("teams: create channel %q: %w", name, err)
	}
	return &Channel{
		ID:   result.ID,
		Name: result.DisplayName,
		URL:  result.WebURL,
	}, nil
}

func (s *TeamsService) PostMessage(channelID string, msg Message) (string, error) {
	var body map[string]interface{}
	if len(msg.Blocks) > 0 {
		// Adaptive Card attachment
		body = map[string]interface{}{
			"body": map[string]string{
				"contentType": "html",
				"content":     "<attachment id=\"0\"></attachment>",
			},
			"attachments": []map[string]interface{}{
				{
					"id":          "0",
					"contentType": "application/vnd.microsoft.card.adaptive",
					"content":     msg.Blocks[0], // caller passes the card JSON
				},
			},
		}
	} else {
		body = map[string]interface{}{
			"body": map[string]string{
				"contentType": "text",
				"content":     msg.Text,
			},
		}
	}

	var result struct {
		ID string `json:"id"`
	}
	url := fmt.Sprintf("%s/teams/%s/channels/%s/messages", graphBaseURL, s.teamID, channelID)
	if err := s.graphPost(url, body, &result); err != nil {
		return "", fmt.Errorf("teams: post message to channel %s: %w", channelID, err)
	}
	return result.ID, nil
}

func (s *TeamsService) UpdateMessage(channelID, messageID string, msg Message) error {
	var body map[string]interface{}
	if len(msg.Blocks) > 0 {
		body = map[string]interface{}{
			"body": map[string]string{
				"contentType": "html",
				"content":     "<attachment id=\"0\"></attachment>",
			},
			"attachments": []map[string]interface{}{
				{
					"id":          "0",
					"contentType": "application/vnd.microsoft.card.adaptive",
					"content":     msg.Blocks[0],
				},
			},
		}
	} else {
		body = map[string]interface{}{
			"body": map[string]string{
				"contentType": "text",
				"content":     msg.Text,
			},
		}
	}

	url := fmt.Sprintf("%s/teams/%s/channels/%s/messages/%s", graphBaseURL, s.teamID, channelID, messageID)
	return s.graphPatch(url, body)
}

func (s *TeamsService) ArchiveChannel(channelID string) error {
	// Graph API does not support archiving individual standard channels (only private channels).
	// Best-effort: rename the channel to indicate it's resolved.
	// This is a known Graph API limitation as of 2025.
	suffix := channelID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	body := map[string]interface{}{
		"displayName": fmt.Sprintf("[RESOLVED] channel-%s", suffix),
	}
	url := fmt.Sprintf("%s/teams/%s/channels/%s", graphBaseURL, s.teamID, channelID)
	if err := s.graphPatch(url, body); err != nil {
		// Log but don't fail — channel rename is best-effort
		slog.Warn("teams: could not rename resolved channel", "channel_id", channelID, "error", err)
	}
	return nil
}

func (s *TeamsService) InviteUsers(channelID string, userIDs []string) error {
	// Teams channels in a Team are visible to all Team members by default.
	// Explicit invites are only needed for private channels.
	// For standard channels, this is a no-op.
	return nil
}

func (s *TeamsService) SendDirectMessage(userID string, msg Message) error {
	// Create or get 1:1 chat with the user, then post
	chat, err := s.createOrGetChat(userID)
	if err != nil {
		return fmt.Errorf("teams: get dm chat with %s: %w", userID, err)
	}
	body := map[string]interface{}{
		"body": map[string]string{
			"contentType": "text",
			"content":     msg.Text,
		},
	}
	url := fmt.Sprintf("%s/chats/%s/messages", graphBaseURL, chat)
	return s.graphPostNoResult(url, body)
}

func (s *TeamsService) GetThreadMessages(channelID, threadTS string) ([]string, error) {
	// threadTS is treated as a message ID in Teams (the root message of the thread)
	url := fmt.Sprintf("%s/teams/%s/channels/%s/messages/%s/replies", graphBaseURL, s.teamID, channelID, threadTS)
	var result struct {
		Value []struct {
			Body struct {
				Content string `json:"content"`
			} `json:"body"`
		} `json:"value"`
	}
	if err := s.graphGet(url, &result); err != nil {
		return nil, fmt.Errorf("teams: get thread replies: %w", err)
	}
	messages := make([]string, 0, len(result.Value))
	for _, v := range result.Value {
		if v.Body.Content != "" {
			messages = append(messages, v.Body.Content)
		}
	}
	return messages, nil
}

// ─── Internal Graph API helpers ───────────────────────────────────────────────

func (s *TeamsService) getTeam() (map[string]interface{}, error) {
	var result map[string]interface{}
	url := fmt.Sprintf("%s/teams/%s", graphBaseURL, s.teamID)
	if err := s.graphGet(url, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *TeamsService) createOrGetChat(userID string) (string, error) {
	body := map[string]interface{}{
		"chatType": "oneOnOne",
		"members": []map[string]interface{}{
			{
				"@odata.type":     "#microsoft.graph.aadUserConversationMember",
				"roles":           []string{"owner"},
				"user@odata.bind": fmt.Sprintf("https://graph.microsoft.com/v1.0/users('%s')", userID),
			},
		},
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := s.graphPost(graphBaseURL+"/chats", body, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

func (s *TeamsService) graphGet(url string, out interface{}) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doRequest(req, out)
}

func (s *TeamsService) graphPost(url string, body interface{}, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doRequest(req, out)
}

func (s *TeamsService) graphPostNoResult(url string, body interface{}) error {
	return s.graphPost(url, body, nil)
}

func (s *TeamsService) graphPatch(url string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doRequest(req, nil)
}

func (s *TeamsService) doRequest(req *http.Request, out interface{}) error {
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("graph API %s %s → %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}
	if out != nil && len(body) > 0 {
		return json.Unmarshal(body, out)
	}
	return nil
}

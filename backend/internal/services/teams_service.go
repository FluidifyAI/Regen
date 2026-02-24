package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

const graphBaseURL = "https://graph.microsoft.com/v1.0"

// TeamsService implements ChatService using Microsoft Graph API for channel management
// and Bot Framework REST API for posting messages to channels.
//
// Why two clients?
//   Graph API (ChannelMessage.Send) only works with delegated (user) auth — app-only
//   tokens get a 403. Bot Framework Proactive Messaging uses a separate OAuth scope
//   (https://api.botframework.com/.default) that does work with client credentials,
//   giving us true Slack parity: same bot, same credentials, no per-channel setup.
//
// Client split:
//   graphClient  — Graph API (create/archive channels, DMs, team info)
//   botfwClient  — Bot Framework REST API (post/update messages in channels)
type TeamsService struct {
	ctx        context.Context
	teamID     string
	tenantID   string
	botAppID   string
	botUserID  string
	serviceURL string // Bot Framework relay URL, e.g. https://smba.trafficmanager.net/amer/

	graphClient *http.Client // Graph API — channel CRUD, DMs
	botfwClient *http.Client // Bot Framework — channel message posting
}

// NewTeamsService creates a TeamsService with two authenticated HTTP clients.
// serviceURL is the Bot Framework relay endpoint for the tenant. It can be found
// in any inbound Bot Framework activity's serviceUrl field. Defaults to the US region
// endpoint if empty; set TEAMS_SERVICE_URL for other regions (EU: smba.trafficmanager.net/emea/).
func NewTeamsService(ctx context.Context, appID, appPassword, tenantID, teamID, botUserID, serviceURL string) (*TeamsService, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)

	// Graph API client — used for channel management and DMs
	graphCfg := clientcredentials.Config{
		ClientID:     appID,
		ClientSecret: appPassword,
		TokenURL:     tokenURL,
		Scopes:       []string{"https://graph.microsoft.com/.default"},
	}
	graphClient := graphCfg.Client(context.Background())
	graphClient.Timeout = 30 * time.Second

	// Bot Framework client — used for posting messages to channels
	// Scope differs from Graph; this is what allows proactive messaging without
	// the delegated-only ChannelMessage.Send permission.
	botfwCfg := clientcredentials.Config{
		ClientID:     appID,
		ClientSecret: appPassword,
		TokenURL:     tokenURL,
		Scopes:       []string{"https://api.botframework.com/.default"},
	}
	botfwClient := botfwCfg.Client(context.Background())
	botfwClient.Timeout = 30 * time.Second

	if serviceURL == "" {
		serviceURL = "https://smba.trafficmanager.net/amer/"
	}
	if !strings.HasSuffix(serviceURL, "/") {
		serviceURL += "/"
	}

	svc := &TeamsService{
		ctx:         ctx,
		teamID:      teamID,
		tenantID:    tenantID,
		botAppID:    appID,
		botUserID:   botUserID,
		serviceURL:  serviceURL,
		graphClient: graphClient,
		botfwClient: botfwClient,
	}

	// Validate Graph credentials by fetching team info
	if _, err := svc.getTeam(); err != nil {
		return nil, fmt.Errorf("teams: credential validation failed (check TEAMS_* env vars and app permissions): %w", err)
	}
	return svc, nil
}

// ─── ChatService implementation ────────────────────────────────────────────────

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

// PostMessage sends a message to a Teams channel via the ChatService interface.
// Internally this creates a new Bot Framework conversation in the channel and posts
// to it. Callers that need the conversationID for future updates should use
// PostToChannel instead.
func (s *TeamsService) PostMessage(channelID string, msg Message) (string, error) {
	_, activityID, err := s.PostToChannel(channelID, msg)
	return activityID, err
}

// UpdateMessage updates an existing message. For Bot Framework updates the caller
// must pass the conversationID as channelID and the activityID as messageID.
// Used by postStatusUpdateToTeams via UpdateConversationMessage.
func (s *TeamsService) UpdateMessage(channelID, messageID string, msg Message) error {
	return s.UpdateConversationMessage(channelID, messageID, msg)
}

// ArchiveChannel best-effort renames the channel to mark it as resolved.
// Graph API does not support archiving standard channels (only private channels),
// so this is a rename-only operation. Always returns nil — the error is logged
// internally. Callers should not check the return value; the incident is already
// resolved regardless of whether the rename succeeds.
func (s *TeamsService) ArchiveChannel(channelID string) error {
	suffix := channelID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	body := map[string]interface{}{
		"displayName": fmt.Sprintf("[RESOLVED] channel-%s", suffix),
	}
	url := fmt.Sprintf("%s/teams/%s/channels/%s", graphBaseURL, s.teamID, channelID)
	if err := s.graphPatch(url, body); err != nil {
		slog.Warn("teams: could not rename resolved channel", "channel_id", channelID, "error", err)
	}
	return nil
}

func (s *TeamsService) InviteUsers(channelID string, userIDs []string) error {
	// Standard channels are visible to all team members; explicit invites are only
	// needed for private channels. No-op for the current standard channel model.
	return nil
}

func (s *TeamsService) SendDirectMessage(userID string, msg Message) error {
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

// ─── Bot Framework channel messaging ──────────────────────────────────────────

// PostToChannel creates a new Bot Framework conversation in the given Teams channel
// and posts a message to it. Returns both the conversationID and activityID so
// callers can store both for future PostToConversation and UpdateConversationMessage calls.
//
// Use this for the initial message when creating an incident channel.
func (s *TeamsService) PostToChannel(teamsChannelID string, msg Message) (conversationID, activityID string, err error) {
	convID, err := s.createChannelConversation(teamsChannelID)
	if err != nil {
		return "", "", fmt.Errorf("teams: create conversation in channel %s: %w", teamsChannelID, err)
	}
	actID, err := s.postToConversation(convID, msg)
	if err != nil {
		return "", "", fmt.Errorf("teams: post to conversation %s: %w", convID, err)
	}
	return convID, actID, nil
}

// PostToConversation posts a new message to an existing Bot Framework conversation.
// Use this for status updates, timeline notes, and bot replies after the initial
// PostToChannel has established the conversationID.
func (s *TeamsService) PostToConversation(conversationID string, msg Message) (string, error) {
	return s.postToConversation(conversationID, msg)
}

// UpdateConversationMessage updates an existing message in a Bot Framework conversation.
// Used to keep the root incident Adaptive Card in sync with the current status.
func (s *TeamsService) UpdateConversationMessage(conversationID, activityID string, msg Message) error {
	url := fmt.Sprintf("%sv3/conversations/%s/activities/%s", s.serviceURL, conversationID, activityID)
	body := buildBotFWMessageBody(msg)
	body["type"] = "message"
	return s.botfwPut(url, body)
}

// createChannelConversation creates a new Bot Framework conversation rooted in a
// Teams channel. This is the mechanism that bypasses ChannelMessage.Send:
// the Bot Framework relay creates the conversation on behalf of the bot.
func (s *TeamsService) createChannelConversation(teamsChannelID string) (string, error) {
	url := fmt.Sprintf("%sv3/conversations", s.serviceURL)
	body := map[string]interface{}{
		"bot": map[string]string{
			"id":   "28:" + s.botAppID,
			"name": "OpenIncident",
		},
		"channelData": map[string]interface{}{
			"channel": map[string]string{"id": teamsChannelID},
			"tenant":  map[string]string{"id": s.tenantID},
		},
		"isGroup": true,
		"members": []interface{}{},
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := s.botfwPostJSON(url, body, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// postToConversation sends a message activity to an existing conversation.
func (s *TeamsService) postToConversation(conversationID string, msg Message) (string, error) {
	url := fmt.Sprintf("%sv3/conversations/%s/activities", s.serviceURL, conversationID)
	body := buildBotFWMessageBody(msg)
	body["type"] = "message"
	var result struct {
		ID string `json:"id"`
	}
	if err := s.botfwPostJSON(url, body, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// buildBotFWMessageBody converts a Message into the Bot Framework activity body.
// Adaptive Cards are sent as attachments; plain text is sent as-is.
func buildBotFWMessageBody(msg Message) map[string]interface{} {
	if len(msg.Blocks) > 0 {
		return map[string]interface{}{
			"attachments": []map[string]interface{}{
				{
					"contentType": "application/vnd.microsoft.card.adaptive",
					"content":     msg.Blocks[0],
				},
			},
		}
	}
	return map[string]interface{}{
		"text": msg.Text,
	}
}

// ─── Graph API helpers ────────────────────────────────────────────────────────

func (s *TeamsService) getTeam() (map[string]interface{}, error) {
	var result map[string]interface{}
	url := fmt.Sprintf("%s/teams/%s", graphBaseURL, s.teamID)
	if err := s.graphGet(url, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *TeamsService) createOrGetChat(userID string) (string, error) {
	members := []map[string]interface{}{
		{
			"@odata.type":     "#microsoft.graph.aadUserConversationMember",
			"roles":           []string{"owner"},
			"user@odata.bind": fmt.Sprintf("https://graph.microsoft.com/v1.0/users('%s')", userID),
		},
	}
	if s.botUserID != "" {
		members = append(members, map[string]interface{}{
			"@odata.type":     "#microsoft.graph.aadUserConversationMember",
			"roles":           []string{"owner"},
			"user@odata.bind": fmt.Sprintf("https://graph.microsoft.com/v1.0/users('%s')", s.botUserID),
		})
	}
	body := map[string]interface{}{
		"chatType": "oneOnOne",
		"members":  members,
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
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doRequest(s.graphClient, req, out)
}

func (s *TeamsService) graphPost(url string, body interface{}, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doRequest(s.graphClient, req, out)
}

func (s *TeamsService) graphPostNoResult(url string, body interface{}) error {
	return s.graphPost(url, body, nil)
}

func (s *TeamsService) graphPatch(url string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doRequest(s.graphClient, req, nil)
}

// ─── Bot Framework API helpers ────────────────────────────────────────────────

func (s *TeamsService) botfwPostJSON(url string, body interface{}, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doRequest(s.botfwClient, req, out)
}

func (s *TeamsService) botfwPut(url string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doRequest(s.botfwClient, req, nil)
}

// ─── Shared HTTP helper ───────────────────────────────────────────────────────

// apiResponseSizeLimit caps response body reads at 4 MB.
// This prevents memory exhaustion if a misconfigured or malicious endpoint
// returns a large error body. Legitimate Graph/Bot Framework responses are well under 1 MB.
const apiResponseSizeLimit = 4 * 1024 * 1024

func (s *TeamsService) doRequest(client *http.Client, req *http.Request, out interface{}) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, apiResponseSizeLimit))
	if err != nil {
		return fmt.Errorf("teams API %s %s: failed to read response body: %w",
			req.Method, req.URL.Path, err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("teams API %s %s → %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}
	if out != nil && len(body) > 0 {
		return json.Unmarshal(body, out)
	}
	return nil
}

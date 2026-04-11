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

	"github.com/FluidifyAI/Regen/backend/internal/models"
)

// TelegramService sends incident notification messages to a Telegram group/channel.
// Notification-only: no channel creation, no inbound command handling in v1.
type TelegramService struct {
	botToken string
	chatID   string
	appURL   string
	client   *http.Client
}

// NewTelegramService constructs a TelegramService. Returns nil if token or chatID is empty.
func NewTelegramService(botToken, chatID, appURL string) *TelegramService {
	if botToken == "" || chatID == "" {
		return nil
	}
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	return &TelegramService{
		botToken: botToken,
		chatID:   chatID,
		appURL:   appURL,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// NewTelegramServiceFromConfig constructs from a DB config row. Returns nil if not configured.
func NewTelegramServiceFromConfig(cfg *models.TelegramConfig, appURL string) *TelegramService {
	if cfg == nil || cfg.BotToken == "" || cfg.ChatID == "" {
		return nil
	}
	return NewTelegramService(cfg.BotToken, cfg.ChatID, appURL)
}

// SendIncidentCreated posts an incident creation notification.
func (s *TelegramService) SendIncidentCreated(incident *models.Incident) error {
	emoji := severityEmoji(string(incident.Severity))
	text := fmt.Sprintf(
		"%s <b>INC-%d — %s</b>\n%s\n\n<a href=\"%s/incidents/%s\">View incident →</a>",
		emoji,
		incident.IncidentNumber,
		telegramToUpper(string(incident.Severity)),
		telegramEscapeHTML(incident.Title),
		s.appURL,
		incident.ID.String(),
	)
	return s.sendMessage(text)
}

// SendStatusUpdate posts a status change notification.
func (s *TelegramService) SendStatusUpdate(incident *models.Incident, newStatus string) error {
	emoji := telegramStatusEmoji(newStatus)
	label := telegramStatusLabel(newStatus)
	text := fmt.Sprintf(
		"%s <b>INC-%d %s</b>\n%s\n\n<a href=\"%s/incidents/%s\">View incident →</a>",
		emoji,
		incident.IncidentNumber,
		label,
		telegramEscapeHTML(incident.Title),
		s.appURL,
		incident.ID.String(),
	)
	return s.sendMessage(text)
}

// TestTelegramConnection verifies the bot token by calling getMe and sends a test message.
func TestTelegramConnection(ctx context.Context, botToken, chatID string) (string, error) {
	c := &http.Client{Timeout: 10 * time.Second}

	// 1. Validate token via getMe
	resp, err := c.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", botToken))
	if err != nil {
		return "", fmt.Errorf("could not reach Telegram API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var getMeResp struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &getMeResp); err != nil || !getMeResp.OK {
		return "", fmt.Errorf("invalid bot token")
	}

	// 2. Send test message to confirm chatID works
	svc := NewTelegramService(botToken, chatID, "")
	if svc == nil {
		return "", fmt.Errorf("bot_token and chat_id are required")
	}
	if err := svc.sendMessage("✅ <b>Fluidify Regen connected</b>\nIncident notifications will appear here."); err != nil {
		return "", fmt.Errorf("bot token valid but could not post to chat: %w", err)
	}

	return getMeResp.Result.Username, nil
}

// FetchTelegramChatID calls getUpdates to find the most recent group chat ID the bot has seen.
// Returns chat_id and chat title of the most recent group/supergroup.
func FetchTelegramChatID(ctx context.Context, botToken string) (string, string, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf(`https://api.telegram.org/bot%s/getUpdates?limit=20&allowed_updates=["message"]`, botToken)
	resp, err := c.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("could not reach Telegram API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))

	var updatesResp struct {
		OK     bool `json:"ok"`
		Result []struct {
			Message struct {
				Chat struct {
					ID    int64  `json:"id"`
					Title string `json:"title"`
					Type  string `json:"type"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &updatesResp); err != nil || !updatesResp.OK {
		return "", "", fmt.Errorf("failed to fetch updates — ensure the bot token is valid")
	}

	// Prefer group/supergroup; fall back to private chat for personal setups
	var privateID, privateName string
	for i := len(updatesResp.Result) - 1; i >= 0; i-- {
		chat := updatesResp.Result[i].Message.Chat
		if chat.Type == "group" || chat.Type == "supergroup" {
			return fmt.Sprintf("%d", chat.ID), chat.Title, nil
		}
		if chat.Type == "private" && privateID == "" {
			privateID = fmt.Sprintf("%d", chat.ID)
			privateName = chat.Title
			if privateName == "" {
				privateName = "Private chat with bot"
			}
		}
	}
	if privateID != "" {
		return privateID, privateName, nil
	}
	return "", "", fmt.Errorf("no messages found — send any message to the bot (or add it to a group and send a message there)")
}

// SendAISummary posts the AI-generated summary for an incident to Telegram.
func (s *TelegramService) SendAISummary(incident *models.Incident, summary string) error {
	emoji := severityEmoji(string(incident.Severity))
	text := fmt.Sprintf(
		"%s <b>INC-%d \u2014 AI Summary</b>\n\n%s\n\n<a href=\"\u0025s/incidents/\u0025s\">View incident</a>",
		emoji,
		incident.IncidentNumber,
		telegramEscapeHTML(summary),
		s.appURL,
		incident.ID.String(),
	)
	return s.sendMessage(text)
}

// SetTelegramService wires the optional Telegram service into the incident service.
// Called by routes.go after construction when Telegram is configured in DB.
func SetTelegramService(svc IncidentService, tg *TelegramService) {
	if is, ok := svc.(*incidentService); ok {
		is.telegramSvc = tg
		slog.Info("telegram notification service wired")
	}
}

// ─── internal helpers ─────────────────────────────────────────────────────────

func (s *TelegramService) sendMessage(htmlText string) error {
	payload := map[string]interface{}{
		"chat_id":    s.chatID,
		"text":       htmlText,
		"parse_mode": "HTML",
		"link_preview_options": map[string]bool{
			"is_disabled": true,
		},
	}
	b, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("telegram sendMessage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("telegram sendMessage: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func severityEmoji(s string) string {
	switch s {
	case "critical":
		return "🔴"
	case "high":
		return "🟠"
	case "medium":
		return "🟡"
	default:
		return "🔵"
	}
}

func telegramStatusEmoji(s string) string {
	switch s {
	case "acknowledged":
		return "🟡"
	case "resolved":
		return "✅"
	case "canceled":
		return "⬜"
	default:
		return "🔴"
	}
}

func telegramStatusLabel(s string) string {
	switch s {
	case "triggered":
		return "Triggered"
	case "acknowledged":
		return "Acknowledged"
	case "resolved":
		return "Resolved"
	case "canceled":
		return "Canceled"
	default:
		return s
	}
}

func telegramToUpper(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 32
	}
	return string(b)
}

// telegramEscapeHTML escapes characters that Telegram HTML mode treats specially.
func telegramEscapeHTML(s string) string {
	r := ""
	for _, c := range s {
		switch c {
		case '<':
			r += "&lt;"
		case '>':
			r += "&gt;"
		case '&':
			r += "&amp;"
		default:
			r += string(c)
		}
	}
	return r
}

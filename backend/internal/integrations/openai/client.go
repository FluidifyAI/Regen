package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.openai.com"

// Client is a minimal OpenAI chat completions client using only net/http.
// No external SDK dependency — keeps the binary lean and the BYO-key model transparent.
type Client struct {
	apiKey     string
	model      string
	maxTokens  int
	baseURL    string
	httpClient *http.Client
}

// New creates a new OpenAI client using the official API endpoint.
func New(apiKey, model string, maxTokens int) *Client {
	return NewWithBaseURL(apiKey, model, maxTokens, defaultBaseURL)
}

// NewWithBaseURL creates a client with a custom base URL (useful for testing).
func NewWithBaseURL(apiKey, model string, maxTokens int, baseURL string) *Client {
	return &Client{
		apiKey:     apiKey,
		model:      model,
		maxTokens:  maxTokens,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// ChatMessage is an OpenAI chat message with role and content.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *openAIError `json:"error"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// Complete sends a chat completion request and returns the assistant reply.
func (c *Client) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	payload := chatCompletionRequest{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: c.maxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	var result chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response (status %d): %w", resp.StatusCode, err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("openai api error [%s]: %s", result.Error.Code, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices (status %d)", resp.StatusCode)
	}

	return result.Choices[0].Message.Content, nil
}

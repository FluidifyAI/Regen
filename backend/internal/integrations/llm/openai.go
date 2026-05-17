package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const openAIDefaultBase = "https://api.openai.com"

type openAIClient struct {
	apiKey     string
	model      string
	maxTokens  int
	baseURL    string
	httpClient *http.Client
}

func newOpenAIClient(cfg Config) *openAIClient {
	base := cfg.BaseURL
	if base == "" {
		base = openAIDefaultBase
	}
	return &openAIClient{
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		maxTokens:  cfg.MaxTokens,
		baseURL:    base,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

type openAIChatRequest struct {
	Model     string              `json:"model"`
	Messages  []openAIChatMessage `json:"messages"`
	MaxTokens int                 `json:"max_tokens"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (c *openAIClient) Complete(ctx context.Context, messages []Message) (string, error) {
	msgs := make([]openAIChatMessage, len(messages))
	for i, m := range messages {
		msgs[i] = openAIChatMessage{Role: m.Role, Content: m.Content}
	}

	body, _ := json.Marshal(openAIChatRequest{Model: c.model, Messages: msgs, MaxTokens: c.maxTokens})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("openai: create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: send request: %w", err)
	}
	defer resp.Body.Close()

	var result openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("openai: decode response (status %d): %w", resp.StatusCode, err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("openai api error [%s]: %s", result.Error.Code, result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response (status %d)", resp.StatusCode)
	}
	return result.Choices[0].Message.Content, nil
}

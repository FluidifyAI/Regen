package llm

import (
	"context"
	"fmt"
)

// Message is a single chat message with a role and content.
type Message struct {
	Role    string
	Content string
}

// Client is the common interface all LLM provider adapters implement.
type Client interface {
	Complete(ctx context.Context, messages []Message) (string, error)
}

// Config holds the provider selection and all provider-specific credentials.
type Config struct {
	Provider  string // "openai" | "anthropic" | "ollama"
	APIKey    string
	Model     string
	BaseURL   string // required for ollama; overrides default for openai/anthropic
	MaxTokens int
}

// New returns a Client for the provider named in cfg.Provider.
func New(cfg Config) (Client, error) {
	switch cfg.Provider {
	case "openai", "":
		return newOpenAIClient(cfg), nil
	case "anthropic":
		return newAnthropicClient(cfg), nil
	case "ollama":
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("llm: ollama provider requires BaseURL (OLLAMA_BASE_URL)")
		}
		return newOllamaClient(cfg), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q (valid: openai, anthropic, ollama)", cfg.Provider)
	}
}

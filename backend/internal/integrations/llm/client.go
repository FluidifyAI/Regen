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

// Usage holds token consumption data returned by an LLM API call.
// Providers that do not return token counts (e.g. Ollama without native API) leave both fields zero.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// Client is the common interface all LLM provider adapters implement.
type Client interface {
	Complete(ctx context.Context, messages []Message) (string, Usage, error)
}

// Config holds the provider selection and all provider-specific credentials.
type Config struct {
	Provider  string // "openai" | "anthropic" | "ollama"
	APIKey    string
	Model     string
	BaseURL   string // required for ollama; overrides default for openai/anthropic
	MaxTokens int
}

// New returns a Client for the provider named in cfg.Provider, wrapped with a
// GenAI span per completion (REG-12) — every provider gets the same
// instrumentation, so callers never need to know or care which one is live.
func New(cfg Config) (Client, error) {
	switch cfg.Provider {
	case "openai", "":
		return newInstrumentedClient(newOpenAIClient(cfg, "openai"), "openai", cfg.Model), nil
	case "anthropic":
		return newInstrumentedClient(newAnthropicClient(cfg), "anthropic", cfg.Model), nil
	case "ollama":
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("llm: ollama provider requires BaseURL (OLLAMA_BASE_URL)")
		}
		return newInstrumentedClient(newOllamaClient(cfg), "ollama", cfg.Model), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q (valid: openai, anthropic, ollama)", cfg.Provider)
	}
}

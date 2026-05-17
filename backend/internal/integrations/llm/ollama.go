package llm

import "context"

// ollamaClient wraps openAIClient — Ollama speaks OpenAI-compat /v1/chat/completions
// with no auth header and a user-supplied base URL.
type ollamaClient struct {
	inner *openAIClient
}

func newOllamaClient(cfg Config) *ollamaClient {
	inner := newOpenAIClient(cfg) // base URL already set from cfg.BaseURL
	inner.apiKey = ""             // no auth for Ollama
	return &ollamaClient{inner: inner}
}

func (c *ollamaClient) Complete(ctx context.Context, messages []Message) (string, error) {
	return c.inner.Complete(ctx, messages)
}

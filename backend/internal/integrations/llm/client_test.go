package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/llm"
)

// ─── OpenAI adapter ──────────────────────────────────────────────────────────

func TestOpenAIComplete_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing Bearer auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "hello openai"}},
			},
		})
	}))
	defer srv.Close()

	c, err := llm.New(llm.Config{Provider: "openai", APIKey: "sk-test", Model: "gpt-4o-mini", MaxTokens: 100, BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := c.Complete(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "hello openai" {
		t.Errorf("got %q, want %q", got, "hello openai")
	}
}

func TestOpenAIComplete_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "invalid key", "type": "auth_error", "code": "invalid_api_key"},
		})
	}))
	defer srv.Close()

	c, _ := llm.New(llm.Config{Provider: "openai", APIKey: "bad", Model: "gpt-4o-mini", MaxTokens: 10, BaseURL: srv.URL})
	_, err := c.Complete(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	if err == nil || !strings.Contains(err.Error(), "invalid key") {
		t.Errorf("expected api error, got %v", err)
	}
}

// ─── Anthropic adapter ───────────────────────────────────────────────────────

func TestAnthropicComplete_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") == "" {
			t.Error("missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}

		// Verify system prompt is top-level, not a message
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["system"] == nil {
			t.Error("system prompt must be a top-level field, not inside messages")
		}
		msgs, _ := body["messages"].([]any)
		for _, m := range msgs {
			msg := m.(map[string]any)
			if msg["role"] == "system" {
				t.Error("system role must not appear inside messages[] for Anthropic")
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "hello anthropic"},
			},
		})
	}))
	defer srv.Close()

	c, err := llm.New(llm.Config{Provider: "anthropic", APIKey: "ant-test", Model: "claude-haiku-4-5-20251001", MaxTokens: 100, BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := c.Complete(context.Background(), []llm.Message{
		{Role: "system", Content: "you are helpful"},
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "hello anthropic" {
		t.Errorf("got %q, want %q", got, "hello anthropic")
	}
}

func TestAnthropicComplete_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"type": "authentication_error", "message": "invalid x-api-key"},
		})
	}))
	defer srv.Close()

	c, _ := llm.New(llm.Config{Provider: "anthropic", APIKey: "bad", Model: "claude-haiku-4-5-20251001", MaxTokens: 10, BaseURL: srv.URL})
	_, err := c.Complete(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for bad Anthropic key")
	}
}

// ─── Ollama adapter ──────────────────────────────────────────────────────────

func TestOllamaComplete_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("Ollama must not send Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "hello ollama"}},
			},
		})
	}))
	defer srv.Close()

	c, err := llm.New(llm.Config{Provider: "ollama", Model: "llama3", MaxTokens: 100, BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := c.Complete(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "hello ollama" {
		t.Errorf("got %q, want %q", got, "hello ollama")
	}
}

// ─── Factory ─────────────────────────────────────────────────────────────────

func TestNew_unknownProvider(t *testing.T) {
	_, err := llm.New(llm.Config{Provider: "grok", APIKey: "x"})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestNew_ollamaRequiresBaseURL(t *testing.T) {
	_, err := llm.New(llm.Config{Provider: "ollama", Model: "llama3"})
	if err == nil {
		t.Error("expected error when Ollama BaseURL is empty")
	}
}

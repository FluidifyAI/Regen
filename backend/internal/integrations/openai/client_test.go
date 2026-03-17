package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluidify/regen/internal/integrations/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "Incident summary here."}},
			},
		})
	}))
	defer server.Close()

	client := openai.NewWithBaseURL("test-key", "gpt-4o-mini", 500, server.URL)
	result, err := client.Complete(context.Background(), []openai.ChatMessage{
		{Role: "user", Content: "Summarize this incident."},
	})

	require.NoError(t, err)
	assert.Equal(t, "Incident summary here.", result)
}

func TestClient_Complete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Incorrect API key",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
	}))
	defer server.Close()

	client := openai.NewWithBaseURL("bad-key", "gpt-4o-mini", 500, server.URL)
	_, err := client.Complete(context.Background(), []openai.ChatMessage{
		{Role: "user", Content: "test"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_api_key")
}

func TestClient_Complete_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{}})
	}))
	defer server.Close()

	client := openai.NewWithBaseURL("test-key", "gpt-4o-mini", 500, server.URL)
	_, err := client.Complete(context.Background(), []openai.ChatMessage{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no choices")
}

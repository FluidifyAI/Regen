package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericProvider_Source(t *testing.T) {
	provider := &GenericProvider{}
	assert.Equal(t, "generic", provider.Source())
}

func TestGenericProvider_ValidatePayload_NoSecret(t *testing.T) {
	provider := &GenericProvider{WebhookSecret: ""}

	// No secret configured, should skip validation
	err := provider.ValidatePayload([]byte("any payload"), http.Header{})
	assert.NoError(t, err)
}

func TestGenericProvider_ValidatePayload_ValidSignature(t *testing.T) {
	secret := "my-webhook-secret"
	provider := &GenericProvider{WebhookSecret: secret}

	payload := []byte(`{"alerts":[{"title":"test"}]}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	headers := http.Header{}
	headers.Set("X-Webhook-Signature", signature)

	err := provider.ValidatePayload(payload, headers)
	assert.NoError(t, err)
}

func TestGenericProvider_ValidatePayload_InvalidSignature(t *testing.T) {
	provider := &GenericProvider{WebhookSecret: "my-webhook-secret"}

	payload := []byte(`{"alerts":[{"title":"test"}]}`)
	headers := http.Header{}
	headers.Set("X-Webhook-Signature", "sha256=invalid-signature")

	err := provider.ValidatePayload(payload, headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature verification failed")
}

func TestGenericProvider_ValidatePayload_MissingSignature(t *testing.T) {
	provider := &GenericProvider{WebhookSecret: "my-webhook-secret"}

	payload := []byte(`{"alerts":[{"title":"test"}]}`)
	headers := http.Header{}

	err := provider.ValidatePayload(payload, headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing X-Webhook-Signature header")
}

func TestGenericProvider_ValidatePayload_InvalidSignatureFormat(t *testing.T) {
	provider := &GenericProvider{WebhookSecret: "my-webhook-secret"}

	payload := []byte(`{"alerts":[{"title":"test"}]}`)
	headers := http.Header{}
	headers.Set("X-Webhook-Signature", "not-sha256-format")

	err := provider.ValidatePayload(payload, headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature format")
}

func TestGenericProvider_ParsePayload_Minimal(t *testing.T) {
	provider := &GenericProvider{}

	// Load minimal example (only title required)
	payload, err := os.ReadFile("testdata/generic-minimal.json")
	require.NoError(t, err)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Verify basic fields
	assert.Equal(t, "generic", alert.Source)
	assert.Equal(t, "High CPU on web-01", alert.Title)
	assert.Equal(t, "", alert.Description)

	// Verify defaults
	assert.Equal(t, "warning", alert.Severity, "should default to warning")
	assert.Equal(t, "firing", alert.Status, "should default to firing")
	assert.NotEmpty(t, alert.ExternalID, "should auto-generate external_id")
	assert.Empty(t, alert.Labels, "should have empty labels map")
	assert.Empty(t, alert.Annotations, "should have empty annotations map")
	assert.Nil(t, alert.EndedAt, "firing alert should not have ended_at")

	// Verify started_at is recent (defaulted to current time)
	assert.WithinDuration(t, time.Now(), alert.StartedAt, 5*time.Second)
}

func TestGenericProvider_ParsePayload_Full(t *testing.T) {
	provider := &GenericProvider{}

	payload, err := os.ReadFile("testdata/generic-full.json")
	require.NoError(t, err)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Verify all fields
	assert.Equal(t, "generic", alert.Source)
	assert.Equal(t, "custom-error-rate-123", alert.ExternalID)
	assert.Equal(t, "critical", alert.Severity)
	assert.Equal(t, "firing", alert.Status)
	assert.Equal(t, "High Error Rate", alert.Title)
	assert.Equal(t, "Error rate exceeded 5% on api-gateway", alert.Description)

	// Verify labels
	assert.Equal(t, "api-gateway", alert.Labels["service"])
	assert.Equal(t, "production", alert.Labels["env"])
	assert.Equal(t, "backend", alert.Labels["team"])

	// Verify annotations
	assert.Equal(t, "https://wiki.example.com/runbooks/error-rate", alert.Annotations["runbook_url"])
	assert.Equal(t, "https://grafana.example.com/d/api-dashboard", alert.Annotations["dashboard_url"])

	// Verify timestamps
	expectedStart := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedStart, alert.StartedAt)
	assert.Nil(t, alert.EndedAt)
}

func TestGenericProvider_ParsePayload_Resolved(t *testing.T) {
	provider := &GenericProvider{}

	payload, err := os.ReadFile("testdata/generic-resolved.json")
	require.NoError(t, err)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Verify resolved status
	assert.Equal(t, "resolved", alert.Status)
	assert.Equal(t, "custom-error-rate-123", alert.ExternalID)

	// Verify ended_at is set
	require.NotNil(t, alert.EndedAt)
	expectedEnd := time.Date(2024, 1, 1, 12, 15, 0, 0, time.UTC)
	assert.Equal(t, expectedEnd, *alert.EndedAt)
}

func TestGenericProvider_ParsePayload_AutoGeneratedExternalID(t *testing.T) {
	provider := &GenericProvider{}

	payload := []byte(`{
		"alerts": [{
			"title": "Test Alert",
			"labels": {"service": "api", "env": "prod"}
		}]
	}`)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Should have auto-generated external_id
	assert.NotEmpty(t, alert.ExternalID)
	assert.Len(t, alert.ExternalID, 32, "generated ID should be 32 chars (SHA256 truncated)")

	// Same payload should produce same external_id (deterministic)
	alerts2, _ := provider.ParsePayload(payload)
	assert.Equal(t, alert.ExternalID, alerts2[0].ExternalID, "should be deterministic")

	// Different payload should produce different external_id
	payload3 := []byte(`{
		"alerts": [{
			"title": "Different Alert",
			"labels": {"service": "api", "env": "prod"}
		}]
	}`)
	alerts3, _ := provider.ParsePayload(payload3)
	assert.NotEqual(t, alert.ExternalID, alerts3[0].ExternalID, "different title should produce different ID")
}

func TestGenericProvider_ParsePayload_InvalidJSON(t *testing.T) {
	provider := &GenericProvider{}

	_, err := provider.ParsePayload([]byte("not json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid generic webhook payload")
}

func TestGenericProvider_ParsePayload_EmptyAlerts(t *testing.T) {
	provider := &GenericProvider{}

	payload := []byte(`{"alerts": []}`)
	_, err := provider.ParsePayload(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contains no alerts")
}

func TestGenericProvider_ParsePayload_InvalidSeverity(t *testing.T) {
	provider := &GenericProvider{}

	payload := []byte(`{
		"alerts": [{
			"title": "Test",
			"severity": "invalid-severity"
		}]
	}`)

	_, err := provider.ParsePayload(payload)
	assert.Error(t, err, "should reject invalid severity")
}

func TestGenericProvider_ParsePayload_InvalidStatus(t *testing.T) {
	provider := &GenericProvider{}

	payload := []byte(`{
		"alerts": [{
			"title": "Test",
			"status": "invalid-status"
		}]
	}`)

	_, err := provider.ParsePayload(payload)
	assert.Error(t, err, "should reject invalid status")
}

func TestGenericProvider_ParsePayload_EndedAtOnFiring(t *testing.T) {
	provider := &GenericProvider{}

	// Invalid: firing alert cannot have ended_at
	payload := []byte(`{
		"alerts": [{
			"title": "Test",
			"status": "firing",
			"ended_at": "2024-01-01T12:00:00Z"
		}]
	}`)

	_, err := provider.ParsePayload(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ended_at cannot be set for firing alerts")
}

func TestGenericProvider_ParsePayload_MultipleAlerts(t *testing.T) {
	provider := &GenericProvider{}

	payload := []byte(`{
		"alerts": [
			{"title": "Alert 1", "severity": "critical"},
			{"title": "Alert 2", "severity": "warning"},
			{"title": "Alert 3", "severity": "info"}
		]
	}`)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 3)

	assert.Equal(t, "Alert 1", alerts[0].Title)
	assert.Equal(t, "critical", alerts[0].Severity)
	assert.Equal(t, "Alert 2", alerts[1].Title)
	assert.Equal(t, "warning", alerts[1].Severity)
	assert.Equal(t, "Alert 3", alerts[2].Title)
	assert.Equal(t, "info", alerts[2].Severity)
}

func TestGetJSONSchema(t *testing.T) {
	schema := GetJSONSchema()

	// Verify top-level schema fields
	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	assert.Equal(t, "Fluidify Regen Generic Webhook", schema["title"])
	assert.Equal(t, "object", schema["type"])

	// Verify required fields
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "alerts")

	// Verify properties exist
	properties, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "alerts")

	// Verify examples exist
	examples, ok := schema["examples"].([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, examples)
}

func TestGenerateExternalID(t *testing.T) {
	// Test deterministic generation
	title := "Test Alert"
	labels := map[string]string{"service": "api", "env": "prod"}

	id1 := GenerateExternalID(title, labels)
	id2 := GenerateExternalID(title, labels)

	assert.Equal(t, id1, id2, "should be deterministic")
	assert.Len(t, id1, 32, "should be 32 chars")

	// Test different inputs produce different IDs
	id3 := GenerateExternalID("Different Title", labels)
	assert.NotEqual(t, id1, id3, "different title should produce different ID")

	id4 := GenerateExternalID(title, map[string]string{"service": "db"})
	assert.NotEqual(t, id1, id4, "different labels should produce different ID")

	// Test label order doesn't matter (sorted internally)
	labels1 := map[string]string{"a": "1", "b": "2", "c": "3"}
	labels2 := map[string]string{"c": "3", "a": "1", "b": "2"}

	id5 := GenerateExternalID(title, labels1)
	id6 := GenerateExternalID(title, labels2)
	assert.Equal(t, id5, id6, "label order should not affect ID")
}

package webhooks

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrafanaProvider_Source(t *testing.T) {
	provider := &GrafanaProvider{}
	assert.Equal(t, "grafana", provider.Source())
}

func TestGrafanaProvider_ValidatePayload(t *testing.T) {
	provider := &GrafanaProvider{}

	// Grafana doesn't validate signatures, should always return nil
	err := provider.ValidatePayload([]byte("any payload"), nil)
	assert.NoError(t, err)
}

func TestGrafanaProvider_ParsePayload_Firing(t *testing.T) {
	provider := &GrafanaProvider{}

	// Load test fixture
	payload, err := os.ReadFile("testdata/grafana-firing.json")
	require.NoError(t, err)

	// Parse payload
	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Verify basic fields
	assert.Equal(t, "grafana", alert.Source)
	assert.Equal(t, "a1b2c3d4e5f6", alert.ExternalID)
	assert.Equal(t, "firing", alert.Status)
	assert.Equal(t, "critical", alert.Severity)
	assert.Equal(t, "High CPU Usage", alert.Title)
	assert.Equal(t, "CPU utilization is above 90%", alert.Description)

	// Verify labels
	assert.Equal(t, "High CPU Usage", alert.Labels["alertname"])
	assert.Equal(t, "critical", alert.Labels["severity"])
	assert.Equal(t, "web-01", alert.Labels["instance"])
	assert.Equal(t, "api-gateway", alert.Labels["service"])
	assert.Equal(t, "production", alert.Labels["env"])

	// Verify annotations (includes Grafana-specific fields)
	assert.Equal(t, "CPU utilization is above 90%", alert.Annotations["summary"])
	assert.Contains(t, alert.Annotations["description"], "API gateway server")
	assert.Equal(t, "[ var='A' labels={instance=web-01} value=95.2 ]", alert.Annotations["grafana_query_result"])
	assert.Equal(t, "https://grafana.example.com/alerting/grafana/abc123/view", alert.Annotations["grafana_url"])

	// Verify timestamps
	expectedStart := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedStart, alert.StartedAt)
	assert.Nil(t, alert.EndedAt, "firing alert should not have ended_at")

	// Verify raw payload is preserved
	assert.NotEmpty(t, alert.RawPayload)
}

func TestGrafanaProvider_ParsePayload_Resolved(t *testing.T) {
	provider := &GrafanaProvider{}

	payload, err := os.ReadFile("testdata/grafana-resolved.json")
	require.NoError(t, err)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Verify status is resolved
	assert.Equal(t, "resolved", alert.Status)
	assert.Equal(t, "a1b2c3d4e5f6", alert.ExternalID, "same fingerprint for update")

	// Verify ended_at is set
	require.NotNil(t, alert.EndedAt)
	expectedEnd := time.Date(2024, 1, 1, 12, 15, 0, 0, time.UTC)
	assert.Equal(t, expectedEnd, *alert.EndedAt)
}

func TestGrafanaProvider_ParsePayload_NoFingerprint(t *testing.T) {
	provider := &GrafanaProvider{}

	// Test case: Grafana payload without fingerprint field
	// Should derive external_id from orgId + ruleUID
	payload, err := os.ReadFile("testdata/grafana-no-fingerprint.json")
	require.NoError(t, err)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Should derive external_id from orgId + ruleUID
	assert.Equal(t, "org1-rule-uid-123", alert.ExternalID)
	assert.Equal(t, "Database Connection Failed", alert.Title)
	assert.Equal(t, "warning", alert.Severity)
}

func TestGrafanaProvider_ParsePayload_NoFingerprintNoOrgId(t *testing.T) {
	provider := &GrafanaProvider{}

	// Test case: No fingerprint AND no orgId/ruleUID
	// Should fall back to generating external_id from title + labels
	payload := []byte(`{
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "Test Alert", "env": "prod"},
			"annotations": {},
			"startsAt": "2024-01-01T00:00:00Z"
		}]
	}`)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Should have generated external_id (deterministic hash)
	assert.NotEmpty(t, alert.ExternalID)
	assert.NotEqual(t, "", alert.ExternalID)

	// Same title + labels should produce same external_id
	alerts2, _ := provider.ParsePayload(payload)
	assert.Equal(t, alert.ExternalID, alerts2[0].ExternalID, "generated external_id should be deterministic")
}

func TestGrafanaProvider_ParsePayload_InvalidJSON(t *testing.T) {
	provider := &GrafanaProvider{}

	_, err := provider.ParsePayload([]byte("not json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid grafana webhook payload")
}

func TestGrafanaProvider_ParsePayload_EmptyAlerts(t *testing.T) {
	provider := &GrafanaProvider{}

	payload := []byte(`{"alerts": []}`)
	_, err := provider.ParsePayload(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contains no alerts")
}

func TestGrafanaProvider_ParsePayload_MultipleAlerts(t *testing.T) {
	provider := &GrafanaProvider{}

	payload := []byte(`{
		"alerts": [
			{
				"status": "firing",
				"labels": {"alertname": "Alert1"},
				"annotations": {},
				"startsAt": "2024-01-01T00:00:00Z",
				"fingerprint": "fp1"
			},
			{
				"status": "firing",
				"labels": {"alertname": "Alert2"},
				"annotations": {},
				"startsAt": "2024-01-01T00:00:00Z",
				"fingerprint": "fp2"
			}
		]
	}`)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 2)

	assert.Equal(t, "Alert1", alerts[0].Title)
	assert.Equal(t, "Alert2", alerts[1].Title)
	assert.Equal(t, "fp1", alerts[0].ExternalID)
	assert.Equal(t, "fp2", alerts[1].ExternalID)
}

func TestGrafanaProvider_ParsePayload_DefaultSeverity(t *testing.T) {
	provider := &GrafanaProvider{}

	// Alert without severity label should default to "warning"
	payload := []byte(`{
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "Test"},
			"annotations": {},
			"startsAt": "2024-01-01T00:00:00Z",
			"fingerprint": "test"
		}]
	}`)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	assert.Equal(t, "warning", alerts[0].Severity)
}

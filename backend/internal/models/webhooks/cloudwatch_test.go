package webhooks

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudWatchProvider_Source(t *testing.T) {
	provider := &CloudWatchProvider{}
	assert.Equal(t, "cloudwatch", provider.Source())
}

func TestCloudWatchProvider_ParsePayload_Alarm(t *testing.T) {
	provider := &CloudWatchProvider{}

	// Load test fixture
	payload, err := os.ReadFile("testdata/cloudwatch-alarm.json")
	require.NoError(t, err)

	// Note: In real usage, ValidatePayload would verify SNS signature
	// For this test, we skip validation and focus on parsing

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Verify basic fields
	assert.Equal(t, "cloudwatch", alert.Source)
	assert.Equal(t, "arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPU", alert.ExternalID)
	assert.Equal(t, "firing", alert.Status)
	assert.Equal(t, "critical", alert.Severity, "ALARM state should map to critical")
	assert.Equal(t, "HighCPU", alert.Title)
	assert.Contains(t, alert.Description, "Threshold Crossed")

	// Verify labels (CloudWatch metadata)
	assert.Equal(t, "HighCPU", alert.Labels["alarm_name"])
	assert.Equal(t, "US East (N. Virginia)", alert.Labels["region"])
	assert.Equal(t, "AWS/EC2", alert.Labels["namespace"])
	assert.Equal(t, "CPUUtilization", alert.Labels["metric_name"])
	assert.Equal(t, "i-0123456789abcdef0", alert.Labels["dimension_InstanceId"])

	// Verify annotations
	assert.Equal(t, "arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPU", alert.Annotations["alarm_arn"])
	assert.Equal(t, "CPU utilization exceeds 80%", alert.Annotations["alarm_description"])
	assert.Equal(t, "OK", alert.Annotations["old_state"])
	assert.Equal(t, "GreaterThanThreshold", alert.Annotations["comparison"])
	assert.Equal(t, "80.00", alert.Annotations["threshold"])

	// Verify timestamps
	expectedStart := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedStart, alert.StartedAt)
	assert.Nil(t, alert.EndedAt, "ALARM state should not have ended_at")

	// Verify raw payload is preserved
	assert.NotEmpty(t, alert.RawPayload)
}

func TestCloudWatchProvider_ParsePayload_OK(t *testing.T) {
	provider := &CloudWatchProvider{}

	payload, err := os.ReadFile("testdata/cloudwatch-ok.json")
	require.NoError(t, err)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Verify status is resolved
	assert.Equal(t, "resolved", alert.Status)
	assert.Equal(t, "info", alert.Severity, "OK state should map to info")
	assert.Equal(t, "arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPU", alert.ExternalID)

	// Verify ended_at is set
	require.NotNil(t, alert.EndedAt)
	expectedEnd := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)
	assert.Equal(t, expectedEnd, *alert.EndedAt)
}

func TestCloudWatchProvider_ParsePayload_InsufficientData(t *testing.T) {
	provider := &CloudWatchProvider{}

	payload := []byte(`{
		"Type": "Notification",
		"MessageId": "test-123",
		"TopicArn": "arn:aws:sns:us-east-1:123:test",
		"Subject": "INSUFFICIENT_DATA: Test Alarm",
		"Message": "{\"AlarmName\":\"TestAlarm\",\"NewStateValue\":\"INSUFFICIENT_DATA\",\"NewStateReason\":\"Not enough data to evaluate alarm\",\"StateChangeTime\":\"2024-01-01T12:00:00.000+0000\",\"AlarmArn\":\"arn:aws:cloudwatch:us-east-1:123:alarm:TestAlarm\",\"OldStateValue\":\"OK\",\"Trigger\":{\"MetricName\":\"TestMetric\",\"Namespace\":\"AWS/Test\",\"Dimensions\":[]}}",
		"Timestamp": "2024-01-01T12:00:00.000Z",
		"SignatureVersion": "1",
		"Signature": "test",
		"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
	}`)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// INSUFFICIENT_DATA should map to firing with info severity
	assert.Equal(t, "firing", alert.Status)
	assert.Equal(t, "info", alert.Severity, "INSUFFICIENT_DATA should be info severity")
	assert.Contains(t, alert.Description, "Not enough data")
}

func TestCloudWatchProvider_ParsePayload_SubscriptionConfirmation(t *testing.T) {
	t.Skip("Skipping: requires HTTP mock for SNS subscription confirmation")

	// Note: This test would need an HTTP mock server to simulate SNS subscription confirmation
	// In production, ParsePayload() makes an HTTP GET to SubscribeURL to confirm subscription
	// For now, we skip this test and rely on manual/integration testing for this flow
	//
	// TODO: Add HTTP mock server or make confirmSubscription() mockable for testing
}

func TestCloudWatchProvider_ParsePayload_InvalidSNS(t *testing.T) {
	provider := &CloudWatchProvider{}

	_, err := provider.ParsePayload([]byte("not json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid SNS message")
}

func TestCloudWatchProvider_ParsePayload_InvalidCloudWatchJSON(t *testing.T) {
	provider := &CloudWatchProvider{}

	// Valid SNS envelope but invalid CloudWatch JSON in Message field
	payload := []byte(`{
		"Type": "Notification",
		"MessageId": "test-123",
		"TopicArn": "arn:aws:sns:us-east-1:123:test",
		"Message": "not valid json",
		"Timestamp": "2024-01-01T12:00:00.000Z",
		"SignatureVersion": "1",
		"Signature": "test",
		"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
	}`)

	_, err := provider.ParsePayload(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CloudWatch alarm")
}

func TestCloudWatchProvider_ParsePayload_UnsupportedMessageType(t *testing.T) {
	provider := &CloudWatchProvider{}

	payload := []byte(`{
		"Type": "UnsubscribeConfirmation",
		"MessageId": "test-123",
		"TopicArn": "arn:aws:sns:us-east-1:123:test",
		"Message": "test",
		"Timestamp": "2024-01-01T12:00:00.000Z",
		"SignatureVersion": "1",
		"Signature": "test",
		"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
	}`)

	_, err := provider.ParsePayload(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported SNS message type")
}

func TestCloudWatchProvider_ParsePayload_UnknownState(t *testing.T) {
	provider := &CloudWatchProvider{}

	payload := []byte(`{
		"Type": "Notification",
		"MessageId": "test-123",
		"TopicArn": "arn:aws:sns:us-east-1:123:test",
		"Message": "{\"AlarmName\":\"Test\",\"NewStateValue\":\"UNKNOWN_STATE\",\"NewStateReason\":\"Test\",\"StateChangeTime\":\"2024-01-01T12:00:00.000+0000\",\"AlarmArn\":\"arn:aws:cloudwatch:us-east-1:123:alarm:Test\",\"Trigger\":{}}",
		"Timestamp": "2024-01-01T12:00:00.000Z",
		"SignatureVersion": "1",
		"Signature": "test",
		"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
	}`)

	_, err := provider.ParsePayload(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown CloudWatch state")
}

func TestCloudWatchProvider_ParsePayload_MultipleDimensions(t *testing.T) {
	provider := &CloudWatchProvider{}

	payload := []byte(`{
		"Type": "Notification",
		"MessageId": "test-123",
		"TopicArn": "arn:aws:sns:us-east-1:123:test",
		"Message": "{\"AlarmName\":\"MultiDimTest\",\"NewStateValue\":\"ALARM\",\"NewStateReason\":\"Test\",\"StateChangeTime\":\"2024-01-01T12:00:00.000+0000\",\"AlarmArn\":\"arn:aws:cloudwatch:us-east-1:123:alarm:Test\",\"Trigger\":{\"Dimensions\":[{\"name\":\"InstanceId\",\"value\":\"i-123\"},{\"name\":\"Region\",\"value\":\"us-east-1\"}]}}",
		"Timestamp": "2024-01-01T12:00:00.000Z",
		"SignatureVersion": "1",
		"Signature": "test",
		"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
	}`)

	alerts, err := provider.ParsePayload(payload)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]

	// Verify multiple dimensions are flattened into labels
	assert.Equal(t, "i-123", alert.Labels["dimension_InstanceId"])
	assert.Equal(t, "us-east-1", alert.Labels["dimension_Region"])
}

// Test helper functions

func TestValidateSigningCertURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		expectErr bool
	}{
		{
			name:      "valid AWS SNS cert URL",
			url:       "https://sns.us-east-1.amazonaws.com/cert.pem",
			expectErr: false,
		},
		{
			name:      "valid AWS SNS cert URL different region",
			url:       "https://sns.eu-west-1.amazonaws.com/cert.pem",
			expectErr: false,
		},
		{
			name:      "non-HTTPS",
			url:       "http://sns.us-east-1.amazonaws.com/cert.pem",
			expectErr: true,
		},
		{
			name:      "non-amazonaws.com domain",
			url:       "https://evil.com/cert.pem",
			expectErr: true,
		},
		{
			name:      "invalid URL",
			url:       "not a url",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSigningCertURL(tt.url)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildCanonicalMessage_Notification(t *testing.T) {
	snsMsg := &SNSMessage{
		Type:      "Notification",
		MessageId: "test-123",
		Message:   "test message",
		Subject:   "test subject",
		Timestamp: "2024-01-01T12:00:00.000Z",
		TopicArn:  "arn:aws:sns:us-east-1:123:test",
	}

	canonical := buildCanonicalMessage(snsMsg)

	// Verify canonical format for Notification messages
	assert.Contains(t, canonical, "Message\ntest message\n")
	assert.Contains(t, canonical, "MessageId\ntest-123\n")
	assert.Contains(t, canonical, "Subject\ntest subject\n")
	assert.Contains(t, canonical, "Timestamp\n2024-01-01T12:00:00.000Z\n")
	assert.Contains(t, canonical, "TopicArn\narn:aws:sns:us-east-1:123:test\n")
	assert.Contains(t, canonical, "Type\nNotification\n")
}

func TestBuildCanonicalMessage_SubscriptionConfirmation(t *testing.T) {
	snsMsg := &SNSMessage{
		Type:         "SubscriptionConfirmation",
		MessageId:    "test-123",
		Message:      "test message",
		Timestamp:    "2024-01-01T12:00:00.000Z",
		TopicArn:     "arn:aws:sns:us-east-1:123:test",
		SubscribeURL: "https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription&Token=test",
	}

	canonical := buildCanonicalMessage(snsMsg)

	// Verify canonical format for SubscriptionConfirmation includes SubscribeURL
	assert.Contains(t, canonical, "SubscribeURL\n")
	assert.Contains(t, canonical, "https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription&Token=test")
}

func TestBuildCanonicalMessage_EmptyFields(t *testing.T) {
	snsMsg := &SNSMessage{
		Type:      "Notification",
		MessageId: "test-123",
		Message:   "test",
		Subject:   "", // Empty subject
		Timestamp: "2024-01-01T12:00:00.000Z",
		TopicArn:  "arn:aws:sns:us-east-1:123:test",
	}

	canonical := buildCanonicalMessage(snsMsg)

	// Empty fields should not be included in canonical message
	assert.NotContains(t, canonical, "Subject\n\n", "empty subject should be excluded")
}

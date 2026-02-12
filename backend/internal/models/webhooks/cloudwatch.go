package webhooks

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SNSMessage represents the envelope sent by AWS Simple Notification Service.
//
// CloudWatch alarms are delivered via SNS, which wraps them in this structure.
// SNS is also used for subscription confirmation - the first message sent when
// configuring the webhook requires an HTTP GET callback to confirm.
//
// Spec: https://docs.aws.amazon.com/sns/latest/dg/sns-message-and-json-formats.html
type SNSMessage struct {
	// Type identifies the message purpose
	// Values: "Notification" (alarm data), "SubscriptionConfirmation", "UnsubscribeConfirmation"
	Type string `json:"Type" binding:"required"`

	// MessageId is a unique identifier for this SNS message
	MessageId string `json:"MessageId" binding:"required"`

	// TopicArn identifies the SNS topic that sent this message
	TopicArn string `json:"TopicArn" binding:"required"`

	// Subject is a short description (appears in email notifications)
	// For CloudWatch: "ALARM: \"AlarmName\" in Region"
	Subject string `json:"Subject"`

	// Message contains the actual payload (JSON-encoded CloudWatch alarm for notifications)
	Message string `json:"Message" binding:"required"`

	// Timestamp when SNS sent this message (ISO 8601)
	Timestamp string `json:"Timestamp" binding:"required"`

	// SignatureVersion is the AWS signature version (currently always "1")
	SignatureVersion string `json:"SignatureVersion" binding:"required"`

	// Signature is the base64-encoded RSA-SHA1 signature
	Signature string `json:"Signature" binding:"required"`

	// SigningCertURL is the URL to the X.509 certificate used for signing
	// MUST be HTTPS and from *.amazonaws.com domain (security check)
	SigningCertURL string `json:"SigningCertURL" binding:"required"`

	// SubscribeURL is the confirmation URL (only present in SubscriptionConfirmation messages)
	// OpenIncident will automatically HTTP GET this URL to confirm the subscription
	SubscribeURL string `json:"SubscribeURL,omitempty"`

	// UnsubscribeURL allows unsubscribing from the topic (we don't use this)
	UnsubscribeURL string `json:"UnsubscribeURL,omitempty"`
}

// CloudWatchAlarm represents the alarm data within the SNS Message field.
//
// This is the actual CloudWatch alarm payload, JSON-encoded inside SNS Message field.
// We parse this after validating the SNS envelope.
//
// Spec: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html
type CloudWatchAlarm struct {
	// AlarmName is the human-readable alarm name (e.g., "HighCPU")
	AlarmName string `json:"AlarmName" binding:"required"`

	// AlarmDescription provides context about the alarm (optional)
	AlarmDescription string `json:"AlarmDescription"`

	// NewStateValue is the current alarm state
	// Values: "ALARM" (threshold breached), "OK" (normal), "INSUFFICIENT_DATA" (not enough data)
	NewStateValue string `json:"NewStateValue" binding:"required,oneof=ALARM OK INSUFFICIENT_DATA"`

	// NewStateReason explains why the state changed
	// Example: "Threshold Crossed: 1 datapoint [95.2 (01/01/24 00:00:00)] was greater than the threshold (80.0)"
	NewStateReason string `json:"NewStateReason" binding:"required"`

	// StateChangeTime is when the alarm changed state (ISO 8601)
	StateChangeTime string `json:"StateChangeTime" binding:"required"`

	// Region is the AWS region (e.g., "us-east-1")
	Region string `json:"Region"`

	// AlarmArn is the globally unique identifier for this alarm
	// Example: "arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPU"
	AlarmArn string `json:"AlarmArn" binding:"required"`

	// OldStateValue is the previous alarm state (before this transition)
	OldStateValue string `json:"OldStateValue"`

	// Trigger contains metric details
	Trigger CloudWatchTrigger `json:"Trigger"`
}

// CloudWatchTrigger contains metric metadata for the alarm.
type CloudWatchTrigger struct {
	// MetricName is the CloudWatch metric being monitored (e.g., "CPUUtilization")
	MetricName string `json:"MetricName"`

	// Namespace is the AWS service namespace (e.g., "AWS/EC2", "AWS/RDS")
	Namespace string `json:"Namespace"`

	// StatisticType is the aggregation method (e.g., "Statistic", "Metric")
	StatisticType string `json:"StatisticType"`

	// Statistic is the aggregation function (e.g., "Average", "Sum", "Maximum")
	Statistic string `json:"Statistic"`

	// Dimensions are key-value tags for the metric
	// Example: [{"name": "InstanceId", "value": "i-0123456789abcdef"}]
	Dimensions []CloudWatchDimension `json:"Dimensions"`

	// Period is the evaluation window in seconds
	Period int `json:"Period"`

	// EvaluationPeriods is how many periods must breach to trigger alarm
	EvaluationPeriods int `json:"EvaluationPeriods"`

	// ComparisonOperator is the threshold comparison (e.g., "GreaterThanThreshold")
	ComparisonOperator string `json:"ComparisonOperator"`

	// Threshold is the numeric threshold value
	Threshold float64 `json:"Threshold"`

	// TreatMissingData defines behavior for missing data (e.g., "notBreaching")
	TreatMissingData string `json:"TreatMissingData"`

	// EvaluateLowSampleCountPercentile for percentile-based alarms
	EvaluateLowSampleCountPercentile string `json:"EvaluateLowSampleCountPercentile"`
}

// CloudWatchDimension is a key-value tag for a metric.
type CloudWatchDimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CloudWatchProvider implements WebhookProvider for AWS CloudWatch alarms via SNS.
//
// CloudWatch does not send webhooks directly - it uses SNS (Simple Notification Service)
// as a transport layer. This provider handles:
//   1. SNS envelope parsing and signature verification
//   2. Automatic SNS subscription confirmation (zero-config setup)
//   3. CloudWatch alarm JSON extraction from SNS Message field
//   4. Normalization of CloudWatch states to firing/resolved
//
// Field Mapping:
//   - Title: AlarmName
//   - Description: NewStateReason (why alarm triggered)
//   - Severity: ALARM→critical, INSUFFICIENT_DATA→info (OK→resolved, doesn't create alert)
//   - Status: ALARM/INSUFFICIENT_DATA→firing, OK→resolved
//   - ExternalID: AlarmArn (globally unique)
//   - Labels: region, namespace, metric_name, dimensions (flattened)
//   - Annotations: alarm_description, state_change_time, comparison, threshold
type CloudWatchProvider struct{}

// Source returns "cloudwatch"
func (c *CloudWatchProvider) Source() string {
	return "cloudwatch"
}

// ValidatePayload verifies the SNS message signature.
//
// AWS SNS signs messages with RSA-SHA1 using X.509 certificates. We validate to ensure
// the message actually came from AWS and wasn't forged.
//
// Validation steps:
//  1. Verify SigningCertURL is HTTPS and from *.amazonaws.com domain (security)
//  2. Download the X.509 certificate from SigningCertURL
//  3. Extract RSA public key from certificate
//  4. Reconstruct the canonical message string (specific fields in specific order)
//  5. Verify RSA-SHA1 signature matches
//
// Spec: https://docs.aws.amazon.com/sns/latest/dg/sns-verify-signature-of-message.html
func (c *CloudWatchProvider) ValidatePayload(body []byte, headers http.Header) error {
	var snsMsg SNSMessage
	if err := json.Unmarshal(body, &snsMsg); err != nil {
		return fmt.Errorf("invalid SNS message: %w", err)
	}

	// Validate SigningCertURL is from AWS (prevent SSRF attacks)
	if err := validateSigningCertURL(snsMsg.SigningCertURL); err != nil {
		return fmt.Errorf("invalid signing cert URL: %w", err)
	}

	// Download the signing certificate
	certPEM, err := downloadCertificate(snsMsg.SigningCertURL)
	if err != nil {
		return fmt.Errorf("failed to download signing certificate: %w", err)
	}

	// Parse certificate and extract public key
	publicKey, err := parsePublicKey(certPEM)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Build canonical message string for signature verification
	// The fields and order are specified by AWS SNS documentation
	canonicalMsg := buildCanonicalMessage(&snsMsg)

	// Decode base64 signature
	signature, err := base64.StdEncoding.DecodeString(snsMsg.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	// Verify RSA-SHA1 signature
	hash := sha1.Sum([]byte(canonicalMsg))
	err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA1, hash[:], signature)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// ParsePayload handles both SNS subscription confirmation and CloudWatch alarm notifications.
//
// Message flow:
//  1. First webhook: Type="SubscriptionConfirmation" → Auto-confirm by HTTP GET to SubscribeURL
//  2. Subsequent webhooks: Type="Notification" → Extract CloudWatch alarm from Message field
//
// For notifications, we parse the CloudWatch alarm JSON and normalize it.
func (c *CloudWatchProvider) ParsePayload(body []byte) ([]NormalizedAlert, error) {
	var snsMsg SNSMessage
	if err := json.Unmarshal(body, &snsMsg); err != nil {
		return nil, fmt.Errorf("invalid SNS message: %w", err)
	}

	// Handle subscription confirmation (first webhook from AWS)
	if snsMsg.Type == "SubscriptionConfirmation" {
		if err := c.confirmSubscription(snsMsg.SubscribeURL); err != nil {
			return nil, fmt.Errorf("failed to confirm SNS subscription: %w", err)
		}
		// Return empty array - no alerts to process for confirmation messages
		// The webhook handler should return 200 OK with zero alerts created
		return []NormalizedAlert{}, nil
	}

	// Only process Notification messages (ignore UnsubscribeConfirmation, etc.)
	if snsMsg.Type != "Notification" {
		return nil, fmt.Errorf("unsupported SNS message type: %s", snsMsg.Type)
	}

	// Parse CloudWatch alarm from SNS Message field
	var alarm CloudWatchAlarm
	if err := json.Unmarshal([]byte(snsMsg.Message), &alarm); err != nil {
		return nil, fmt.Errorf("invalid CloudWatch alarm in SNS message: %w", err)
	}

	// Normalize the CloudWatch alarm
	normalized, err := c.normalizeCloudWatchAlarm(&alarm)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize CloudWatch alarm: %w", err)
	}

	// CloudWatch sends one alarm per SNS message (unlike Prometheus/Grafana batching)
	return []NormalizedAlert{*normalized}, nil
}

// confirmSubscription confirms an SNS subscription by HTTP GET to the SubscribeURL.
//
// This is required for the SNS topic to start sending notifications. AWS sends a
// SubscriptionConfirmation message on first webhook setup, and we must GET the
// SubscribeURL to complete the handshake.
//
// This provides zero-config setup - users just add the webhook URL to SNS, we handle confirmation.
func (c *CloudWatchProvider) confirmSubscription(subscribeURL string) error {
	// Validate URL is HTTPS and from amazonaws.com (security)
	parsedURL, err := url.Parse(subscribeURL)
	if err != nil {
		return fmt.Errorf("invalid subscribe URL: %w", err)
	}
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("subscribe URL must be HTTPS")
	}
	if !strings.HasSuffix(parsedURL.Host, ".amazonaws.com") {
		return fmt.Errorf("subscribe URL must be from amazonaws.com domain")
	}

	// HTTP GET to confirm subscription
	resp, err := http.Get(subscribeURL)
	if err != nil {
		return fmt.Errorf("failed to GET subscribe URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("subscription confirmation failed with status %d", resp.StatusCode)
	}

	return nil
}

// normalizeCloudWatchAlarm converts a CloudWatch alarm to NormalizedAlert.
//
// State mapping:
//   - ALARM → firing (severity: critical)
//   - OK → resolved (this updates existing alert to resolved state)
//   - INSUFFICIENT_DATA → firing (severity: info, to track data gaps)
func (c *CloudWatchProvider) normalizeCloudWatchAlarm(alarm *CloudWatchAlarm) (*NormalizedAlert, error) {
	// Map CloudWatch state to status and severity
	var status, severity string
	switch alarm.NewStateValue {
	case "ALARM":
		status = "firing"
		severity = "critical"
	case "OK":
		status = "resolved"
		severity = "info" // Resolved alerts keep original severity, but OK is informational
	case "INSUFFICIENT_DATA":
		status = "firing"
		severity = "info" // Not critical, just tracking data gaps
	default:
		return nil, fmt.Errorf("unknown CloudWatch state: %s", alarm.NewStateValue)
	}

	// Parse state change time
	// CloudWatch uses a custom timestamp format with milliseconds and timezone offset
	// Example: "2024-01-01T12:00:00.000+0000"
	stateChangeTime, err := time.Parse("2006-01-02T15:04:05.000-0700", alarm.StateChangeTime)
	if err != nil {
		// Try RFC3339 format as fallback
		stateChangeTime, err = time.Parse(time.RFC3339, alarm.StateChangeTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse StateChangeTime %q: %w", alarm.StateChangeTime, err)
		}
	}
	// Convert to UTC for consistency
	stateChangeTime = stateChangeTime.UTC()

	// Determine started_at and ended_at
	var startedAt time.Time
	var endedAt *time.Time
	if status == "resolved" {
		// For resolved state, this is the resolution time
		endedAt = &stateChangeTime
		// We don't know the original start time, use current time as placeholder
		// In practice, this will update an existing alert that has the real start time
		startedAt = stateChangeTime
	} else {
		// For firing/insufficient_data, this is the start time
		startedAt = stateChangeTime
		endedAt = nil
	}

	// Build labels from CloudWatch metadata
	labels := map[string]string{
		"alarm_name": alarm.AlarmName,
	}
	if alarm.Region != "" {
		labels["region"] = alarm.Region
	}
	if alarm.Trigger.Namespace != "" {
		labels["namespace"] = alarm.Trigger.Namespace
	}
	if alarm.Trigger.MetricName != "" {
		labels["metric_name"] = alarm.Trigger.MetricName
	}

	// Flatten dimensions into labels (e.g., dimension_InstanceId=i-xxx)
	for _, dim := range alarm.Trigger.Dimensions {
		key := fmt.Sprintf("dimension_%s", dim.Name)
		labels[key] = dim.Value
	}

	// Build annotations with additional metadata
	annotations := map[string]string{
		"alarm_arn":        alarm.AlarmArn,
		"state_reason":     alarm.NewStateReason,
		"old_state":        alarm.OldStateValue,
		"state_change_time": alarm.StateChangeTime,
	}
	if alarm.AlarmDescription != "" {
		annotations["alarm_description"] = alarm.AlarmDescription
	}
	if alarm.Trigger.ComparisonOperator != "" {
		annotations["comparison"] = alarm.Trigger.ComparisonOperator
		annotations["threshold"] = fmt.Sprintf("%.2f", alarm.Trigger.Threshold)
	}

	// Marshal CloudWatch alarm as raw payload
	rawPayloadBytes, err := json.Marshal(alarm)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CloudWatch alarm for raw payload: %w", err)
	}

	// Title: alarm name
	title := alarm.AlarmName

	// Description: state reason (explains why alarm triggered)
	description := alarm.NewStateReason

	return &NormalizedAlert{
		ExternalID:  alarm.AlarmArn, // ARN is globally unique
		Source:      "cloudwatch",
		Status:      status,
		Severity:    severity,
		Title:       title,
		Description: description,
		Labels:      labels,
		Annotations: annotations,
		RawPayload:  json.RawMessage(rawPayloadBytes),
		StartedAt:   startedAt,
		EndedAt:     endedAt,
	}, nil
}

// Helper functions for SNS signature verification

// validateSigningCertURL ensures the signing certificate URL is from AWS.
//
// Security: Prevents SSRF attacks where attacker provides malicious cert URL.
// We only download certificates from *.amazonaws.com over HTTPS.
func validateSigningCertURL(certURL string) error {
	parsedURL, err := url.Parse(certURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Must be HTTPS
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("certificate URL must be HTTPS")
	}

	// Must be from amazonaws.com domain
	if !strings.HasSuffix(parsedURL.Host, ".amazonaws.com") {
		return fmt.Errorf("certificate URL must be from amazonaws.com domain, got: %s", parsedURL.Host)
	}

	return nil
}

// downloadCertificate fetches the X.509 certificate from AWS.
func downloadCertificate(certURL string) ([]byte, error) {
	resp, err := http.Get(certURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download certificate: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// parsePublicKey extracts the RSA public key from a PEM-encoded X.509 certificate.
func parsePublicKey(certPEM []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X.509 certificate: %w", err)
	}

	publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("certificate public key is not RSA")
	}

	return publicKey, nil
}

// buildCanonicalMessage constructs the message string for signature verification.
//
// AWS SNS signs a specific string format with specific fields in specific order.
// The order differs between Notification and SubscriptionConfirmation messages.
//
// Spec: https://docs.aws.amazon.com/sns/latest/dg/sns-verify-signature-of-message.html
func buildCanonicalMessage(snsMsg *SNSMessage) string {
	var fields []string

	// Field order for Notification messages
	if snsMsg.Type == "Notification" {
		fields = []string{
			"Message", snsMsg.Message,
			"MessageId", snsMsg.MessageId,
			"Subject", snsMsg.Subject,
			"Timestamp", snsMsg.Timestamp,
			"TopicArn", snsMsg.TopicArn,
			"Type", snsMsg.Type,
		}
	} else {
		// Field order for SubscriptionConfirmation and UnsubscribeConfirmation
		fields = []string{
			"Message", snsMsg.Message,
			"MessageId", snsMsg.MessageId,
			"SubscribeURL", snsMsg.SubscribeURL,
			"Timestamp", snsMsg.Timestamp,
			"Token", snsMsg.SubscribeURL, // Token field equals SubscribeURL for confirmation messages
			"TopicArn", snsMsg.TopicArn,
			"Type", snsMsg.Type,
		}
	}

	// Build string: "Key\nValue\n" for each field
	var canonical strings.Builder
	for i := 0; i < len(fields); i += 2 {
		key := fields[i]
		value := fields[i+1]
		if value != "" { // Only include non-empty fields
			canonical.WriteString(key)
			canonical.WriteString("\n")
			canonical.WriteString(value)
			canonical.WriteString("\n")
		}
	}

	return canonical.String()
}

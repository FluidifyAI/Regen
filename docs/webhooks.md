# Webhook Integration Guide

OpenIncident v0.3+ supports webhooks from multiple monitoring sources. This document explains how to configure each source to send alerts to OpenIncident.

---

## Supported Sources

| Source | Authentication | Batch Support | Auto-Confirmation |
|--------|---------------|---------------|-------------------|
| **Prometheus** | URL secrecy | Yes (up to 100) | N/A |
| **Grafana** | URL secrecy | Yes (up to 100) | N/A |
| **CloudWatch** | SNS signature | No (1 per webhook) | Yes (automatic) |
| **Generic** | Optional HMAC | Yes (up to 100) | N/A |

---

## Prometheus Alertmanager

### Configuration

Add OpenIncident webhook to your Alertmanager configuration:

```yaml
# alertmanager.yml
route:
  receiver: 'openincident'
  group_wait: 10s
  group_interval: 5m
  repeat_interval: 4h

receivers:
  - name: 'openincident'
    webhook_configs:
      - url: 'http://openincident.example.com/api/v1/webhooks/prometheus'
        send_resolved: true  # Send resolved notifications
```

### Example Payload

```json
{
  "version": "4",
  "groupKey": "{}:{alertname=\"HighCPU\"}",
  "status": "firing",
  "receiver": "openincident",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighCPU",
        "severity": "critical",
        "instance": "web-01",
        "job": "node-exporter"
      },
      "annotations": {
        "summary": "High CPU usage detected",
        "description": "CPU usage is above 80% for 5 minutes"
      },
      "startsAt": "2024-01-01T12:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus:9090/graph?...",
      "fingerprint": "a1b2c3d4e5f6g7h8"
    }
  ]
}
```

### Testing

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @backend/internal/models/webhooks/testdata/alertmanager-firing.json
```

---

## Grafana Unified Alerting

### Configuration

1. Navigate to **Alerting → Contact points** in Grafana
2. Click **Add contact point**
3. Select **Webhook** as the contact point type
4. Configure:
   - **Name**: OpenIncident
   - **URL**: `http://openincident.example.com/api/v1/webhooks/grafana`
   - **HTTP Method**: POST
5. Click **Test** to verify connectivity
6. Click **Save contact point**
7. Update your notification policies to use this contact point

### Example Payload

```json
{
  "receiver": "OpenIncident",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "Database Connection Failed",
        "severity": "critical",
        "service": "api-gateway"
      },
      "annotations": {
        "summary": "Cannot connect to PostgreSQL",
        "runbook_url": "https://wiki.example.com/runbooks/db-connection"
      },
      "startsAt": "2024-01-01T12:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://grafana.example.com/alerting/grafana/abc123/view",
      "fingerprint": "a1b2c3d4",
      "values": {
        "A": 0
      },
      "valueString": "[ var='A' labels={service=api-gateway} value=0 ]"
    }
  ],
  "externalURL": "https://grafana.example.com"
}
```

### Testing

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/grafana \
  -H "Content-Type: application/json" \
  -d @backend/internal/models/webhooks/testdata/grafana-firing.json
```

---

## AWS CloudWatch

### Configuration

CloudWatch alarms are delivered via SNS (Simple Notification Service). The webhook automatically confirms SNS subscriptions.

#### Step 1: Create SNS Topic

```bash
aws sns create-topic --name cloudwatch-alarms-openincident
```

#### Step 2: Subscribe OpenIncident Webhook

```bash
aws sns subscribe \
  --topic-arn arn:aws:sns:us-east-1:123456789012:cloudwatch-alarms-openincident \
  --protocol https \
  --notification-endpoint https://openincident.example.com/api/v1/webhooks/cloudwatch
```

**Note:** OpenIncident will automatically confirm the subscription. Check logs for confirmation success.

#### Step 3: Configure CloudWatch Alarm to Use SNS Topic

```bash
aws cloudwatch put-metric-alarm \
  --alarm-name HighCPU \
  --alarm-description "CPU usage exceeds 80%" \
  --metric-name CPUUtilization \
  --namespace AWS/EC2 \
  --statistic Average \
  --period 300 \
  --evaluation-periods 2 \
  --threshold 80 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=InstanceId,Value=i-0123456789abcdef0 \
  --alarm-actions arn:aws:sns:us-east-1:123456789012:cloudwatch-alarms-openincident
```

Or via AWS Console:
1. Navigate to **CloudWatch → Alarms → Create alarm**
2. Select metric (e.g., EC2 CPUUtilization)
3. Configure threshold
4. Under **Notifications**, select your SNS topic
5. Create alarm

### SNS Message Format

CloudWatch alarms are wrapped in an SNS envelope:

```json
{
  "Type": "Notification",
  "MessageId": "a1b2c3d4-...",
  "TopicArn": "arn:aws:sns:us-east-1:123456789012:cloudwatch-alarms",
  "Subject": "ALARM: \"HighCPU\" in US East (N. Virginia)",
  "Message": "{\"AlarmName\":\"HighCPU\",\"NewStateValue\":\"ALARM\",...}",
  "Timestamp": "2024-01-01T12:00:00.000Z",
  "SignatureVersion": "1",
  "Signature": "...",
  "SigningCertURL": "https://sns.us-east-1.amazonaws.com/..."
}
```

The `Message` field contains the actual CloudWatch alarm JSON.

### State Mapping

| CloudWatch State | OpenIncident Status | Severity |
|-----------------|---------------------|----------|
| ALARM | firing | critical |
| OK | resolved | info |
| INSUFFICIENT_DATA | firing | info |

### Testing

CloudWatch/SNS signature verification is enforced. Use actual AWS SNS messages for testing.

---

## Generic Webhook

The Generic webhook is OpenIncident's native format. Use it for:
- Custom monitoring scripts
- AWS Lambda functions
- Internal tools
- Manual testing

### Basic Usage

**Minimal payload (only title required):**

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/generic \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [
      {"title": "High CPU on web-01"}
    ]
  }'
```

**Full payload:**

```json
{
  "alerts": [
    {
      "title": "High Error Rate",
      "description": "Error rate exceeded 5% on api-gateway",
      "severity": "critical",
      "status": "firing",
      "external_id": "custom-error-rate-123",
      "labels": {
        "service": "api-gateway",
        "env": "production",
        "team": "backend"
      },
      "annotations": {
        "runbook_url": "https://wiki.example.com/runbooks/error-rate",
        "dashboard_url": "https://grafana.example.com/d/api-dashboard"
      },
      "started_at": "2024-01-01T12:00:00Z"
    }
  ]
}
```

### Field Reference

| Field | Required | Default | Values | Description |
|-------|----------|---------|--------|-------------|
| `title` | ✅ Yes | - | string | Short alert summary |
| `description` | No | `""` | string | Detailed context |
| `severity` | No | `"warning"` | `critical`, `warning`, `info` | Alert severity |
| `status` | No | `"firing"` | `firing`, `resolved` | Alert state |
| `external_id` | No | Auto-generated | string | Deduplication key |
| `labels` | No | `{}` | map[string]string | Filterable metadata |
| `annotations` | No | `{}` | map[string]string | Display-only metadata |
| `started_at` | No | Current time | ISO 8601 | When alert started |
| `ended_at` | No | `null` | ISO 8601 | When resolved (only if `status=resolved`) |

### Optional HMAC Authentication

To secure your generic webhook with HMAC-SHA256 signatures:

#### 1. Configure webhook secret

```bash
# docker-compose.yml
environment:
  WEBHOOK_SECRET: "your-secure-random-secret-here"
```

#### 2. Send signature with requests

```python
import hmac
import hashlib
import json
import requests

payload = {"alerts": [{"title": "Test Alert"}]}
payload_bytes = json.dumps(payload).encode('utf-8')

secret = b"your-secure-random-secret-here"
signature = hmac.new(secret, payload_bytes, hashlib.sha256).hexdigest()

headers = {
    "Content-Type": "application/json",
    "X-Webhook-Signature": f"sha256={signature}"
}

requests.post(
    "http://localhost:8080/api/v1/webhooks/generic",
    headers=headers,
    data=payload_bytes
)
```

### JSON Schema

Get the full JSON Schema for validation:

```bash
curl http://localhost:8080/api/v1/webhooks/generic/schema
```

Use with tools like:
- **Postman**: Import schema for auto-completion
- **Ajv**: Client-side JSON validation
- **IDEs**: Schema-based autocomplete in VSCode, IntelliJ

---

## Response Format

All webhook endpoints return the same response format:

### Success (200 OK)

```json
{
  "source": "grafana",
  "received": 3,
  "created": 2,
  "updated": 1,
  "incidents_created": 2
}
```

| Field | Description |
|-------|-------------|
| `source` | Webhook source (prometheus, grafana, cloudwatch, generic) |
| `received` | Total alerts in payload |
| `created` | New alerts created |
| `updated` | Existing alerts updated (deduplication) |
| `incidents_created` | New incidents auto-created |

### Error Responses

#### 400 Bad Request
```json
{
  "error": "invalid payload",
  "source": "grafana",
  "detail": "alerts field is required"
}
```

**Causes:**
- Malformed JSON
- Missing required fields
- Invalid field values

#### 401 Unauthorized
```json
{
  "error": "authentication failed",
  "source": "cloudwatch",
  "detail": "signature verification failed"
}
```

**Causes:**
- CloudWatch: Invalid SNS signature
- Generic: Invalid HMAC signature or missing signature when secret is configured

#### 500 Internal Server Error
```json
{
  "error": "processing failed",
  "source": "prometheus"
}
```

**Causes:**
- Database connection failure
- Internal processing error
- Check server logs for details

---

## Deduplication

Alerts are deduplicated using `(source, external_id)` composite key:

| Source | external_id |
|--------|-------------|
| Prometheus | `fingerprint` field from Alertmanager |
| Grafana | `fingerprint` field (or `orgId-ruleUID` if fingerprint missing) |
| CloudWatch | `AlarmArn` (globally unique) |
| Generic | User-provided `external_id` or SHA256(title + sorted labels) |

**Example:** Same Prometheus alert firing twice → first creates alert, second updates existing alert (no duplicate).

---

## Troubleshooting

### Webhook not creating incidents

**Check:**
1. Alert severity: Only `critical` and `warning` create incidents by default
2. Logs: `docker-compose logs -f backend | grep webhook`
3. Database: `SELECT * FROM alerts ORDER BY received_at DESC LIMIT 10;`

### CloudWatch SNS subscription not confirming

**Check:**
1. Webhook must be publicly accessible via HTTPS
2. Check logs: `grep "SNS subscription" docker-compose logs backend`
3. Verify SigningCertURL is from amazonaws.com domain

### Generic webhook signature fails

**Verify:**
1. Secret matches on both sides
2. Signature format: `sha256=<hex-encoded-hmac>`
3. Sign the raw request body (not pretty-printed JSON)

### Grafana alerts not appearing

**Check:**
1. Grafana webhook is using `POST` method (not GET)
2. Content-Type is `application/json`
3. Test contact point in Grafana UI shows success
4. Check notification policy routes to the contact point

---

## Example curl Commands

```bash
# Prometheus
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-payload.json

# Grafana
curl -X POST http://localhost:8080/api/v1/webhooks/grafana \
  -H "Content-Type: application/json" \
  -d @grafana-payload.json

# Generic (minimal)
curl -X POST http://localhost:8080/api/v1/webhooks/generic \
  -H "Content-Type: application/json" \
  -d '{"alerts":[{"title":"Test Alert"}]}'

# Generic (with HMAC)
curl -X POST http://localhost:8080/api/v1/webhooks/generic \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: sha256=abc123..." \
  -d @generic-payload.json

# Get generic schema
curl http://localhost:8080/api/v1/webhooks/generic/schema | jq .
```

---

## Rate Limits

- **Max payload size**: 1MB per webhook request
- **Max alerts per payload**: 100 alerts
- **No rate limiting** by default (configure via reverse proxy if needed)

---

## Next Steps

- [API Documentation](API.md) - Full REST API reference
- [Architecture](ARCHITECTURE.md) - System design details
- [Grouping Rules](grouping-rules.md) - Configure alert grouping (v0.3+)
- [Routing Rules](routing-rules.md) - Configure alert routing (v0.3+)

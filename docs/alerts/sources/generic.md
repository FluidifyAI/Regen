# Generic Webhook

The generic webhook accepts a standardized JSON schema that any monitoring tool, script, or custom integration can send to. Use this for tools not natively supported.

## Webhook URL

```
POST https://your-domain.com/api/v1/webhooks/generic
```

## Schema

```json
{
  "alerts": [
    {
      "title": "Redis memory usage above 90%",
      "description": "Redis instance redis-primary is using 91.2% of available memory.",
      "severity": "critical",
      "status": "firing",
      "external_id": "redis-memory-alert-001",
      "source": "my-monitoring-script",
      "labels": {
        "service": "redis",
        "environment": "production",
        "region": "us-east-1"
      },
      "annotations": {
        "runbook": "https://wiki.yourcompany.com/runbooks/redis-memory",
        "dashboard": "https://grafana.yourcompany.com/d/redis"
      },
      "started_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | Yes | Alert title shown in the UI and Slack |
| `description` | string | No | Detailed description of the alert |
| `severity` | string | No | `critical`, `warning`, `info` — defaults to `warning` |
| `status` | string | No | `firing` or `resolved` — defaults to `firing` |
| `external_id` | string | No | Stable ID for deduplication. If the same `external_id` is sent again, Regen updates the existing alert rather than creating a new one |
| `source` | string | No | Name of the source system |
| `labels` | object | No | Key-value pairs for filtering and routing |
| `annotations` | object | No | Additional metadata (runbook links, dashboards, etc.) |
| `started_at` | string | No | ISO8601 timestamp — defaults to current time |

## Authentication (optional)

Sign requests with an HMAC-SHA256 signature to prevent unauthorized alert injection. Configure a secret under **Settings → Integrations → Generic Webhook**.

When configured, include the header:

```
X-Regen-Signature: sha256=<hmac_hex>
```

The HMAC is computed over the raw request body using your configured secret.

Example in Python:

```python
import hmac, hashlib, json, requests

secret = b"your-webhook-secret"
payload = json.dumps({"alerts": [...]}).encode()
sig = hmac.new(secret, payload, hashlib.sha256).hexdigest()

requests.post(
    "https://your-domain.com/api/v1/webhooks/generic",
    data=payload,
    headers={
        "Content-Type": "application/json",
        "X-Regen-Signature": f"sha256={sig}",
    }
)
```

## Schema reference

Fetch the full JSON Schema at runtime:

```
GET https://your-domain.com/api/v1/webhooks/generic/schema
```

## Sending a resolve

To resolve an alert, send the same `external_id` with `status: resolved`:

```json
{
  "alerts": [
    {
      "external_id": "redis-memory-alert-001",
      "title": "Redis memory usage above 90%",
      "status": "resolved"
    }
  ]
}
```

Regen matches on `external_id` and closes the alert. If all alerts linked to an incident are resolved, the incident is flagged for resolution.

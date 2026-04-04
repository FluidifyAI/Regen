# Prometheus / Alertmanager

Fluidify Regen receives alerts from Prometheus Alertmanager via webhook. When an alert fires, Regen creates an incident, opens a Slack channel, and starts the timeline.

## Webhook URL

```
POST https://your-domain.com/api/v1/webhooks/prometheus
```

No authentication token is required. Security relies on keeping the URL private and network-level access controls.

## Alertmanager configuration

Add a receiver to your `alertmanager.yml`:

```yaml
receivers:
  - name: fluidify-regen
    webhook_configs:
      - url: 'http://localhost:8080/api/v1/webhooks/prometheus'
        send_resolved: true
```

Then add it to a route:

```yaml
route:
  group_by: ['alertname', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: fluidify-regen
  routes:
    - matchers:
        - severity =~ "critical|warning"
      receiver: fluidify-regen
```

Reload Alertmanager after saving:

```bash
curl -X POST http://localhost:9093/-/reload
```

## How alert fields map to Regen

| Alertmanager field | Regen field | Notes |
|---|---|---|
| `labels.alertname` | Title | — |
| `annotations.summary` | Description | Falls back to `annotations.description` |
| `labels.severity` | Severity | `critical`, `warning`, `info` — defaults to `warning` if missing |
| `status` | Status | `firing` → active alert, `resolved` → auto-resolves |
| `fingerprint` | ExternalID | Used for deduplication across fire/resolve cycles |
| All `labels` | Labels | Stored and searchable |
| All `annotations` | Annotations | Stored and searchable |

## Example payload

This is what Alertmanager sends to Regen:

```json
{
  "version": "4",
  "groupKey": "{}:{alertname=\"HighErrorRate\"}",
  "status": "firing",
  "receiver": "fluidify-regen",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighErrorRate",
        "severity": "critical",
        "service": "payments-api",
        "env": "production"
      },
      "annotations": {
        "summary": "Error rate above 5% for payments-api",
        "description": "Current error rate: 8.3%"
      },
      "startsAt": "2024-01-15T10:30:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus:9090/graph?...",
      "fingerprint": "a1b2c3d4e5f6a7b8"
    }
  ]
}
```

## Deduplication

Regen uses the `fingerprint` field to deduplicate. If the same alert fires multiple times before resolving, Regen updates the existing alert rather than creating a new incident.

When Alertmanager sends `status: resolved`, Regen:
1. Marks the alert as resolved
2. If all linked alerts are resolved, suggests resolving the incident (timeline entry added)

## Severity mapping

| Alertmanager `severity` label | Regen severity |
|-------------------------------|----------------|
| `critical` | Critical |
| `warning` | Warning / Medium |
| `info` or `low` | Info / Low |
| *(anything else)* | Warning (default) |

## Incident auto-creation

By default, alerts with severity `critical` or `warning` automatically create an incident. You can configure this under **Settings → Routing Rules**.

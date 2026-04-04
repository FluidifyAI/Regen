# Grafana

Fluidify Regen supports Grafana Unified Alerting (Grafana v9+). Grafana uses a webhook format similar to Prometheus Alertmanager with some additional fields.

## Webhook URL

```
POST https://your-domain.com/api/v1/webhooks/grafana
```

## Grafana configuration

1. In Grafana, go to **Alerting → Contact points**
2. Click **Add contact point**
3. Set **Name** to `Fluidify Regen`
4. Set **Integration** to `Webhook`
5. Set **URL** to your webhook URL
6. Click **Test** to send a test alert
7. Click **Save contact point**

Then add it to a notification policy:

1. Go to **Alerting → Notification policies**
2. Edit the default policy or add a new one
3. Set **Default contact point** to `Fluidify Regen`

## How alert fields map to Regen

| Grafana field | Regen field | Notes |
|---|---|---|
| `alerts[].labels.alertname` | Title | — |
| `alerts[].annotations.summary` | Description | Falls back to `annotations.description` |
| `alerts[].labels.severity` | Severity | `critical`, `warning`, `info` |
| `alerts[].status` | Status | `firing` or `resolved` |
| `alerts[].fingerprint` | ExternalID | Used for deduplication |
| `alerts[].values` | Annotations | Query results stored (e.g. `{"A": 95.2}`) |

## Example payload

```json
{
  "receiver": "fluidify-regen",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighCPUUsage",
        "severity": "warning",
        "grafana_folder": "Infrastructure"
      },
      "annotations": {
        "summary": "CPU usage above 85% on web-01"
      },
      "startsAt": "2024-01-15T10:30:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://grafana.yourcompany.com/alerting/...",
      "fingerprint": "abc123def456",
      "values": {
        "cpu_usage": 87.3
      },
      "valueString": "[ var='A' labels={instance='web-01'} value=87.3 ]"
    }
  ],
  "groupLabels": { "alertname": "HighCPUUsage" },
  "commonLabels": { "severity": "warning" },
  "commonAnnotations": {},
  "externalURL": "https://grafana.yourcompany.com"
}
```

## Legacy Grafana alerting (v8 and below)

Grafana v8 and below use a different payload format. Use the **Generic webhook** source instead:

```
POST https://your-domain.com/api/v1/webhooks/generic
```

See [Generic webhook](./generic.md) for the schema.

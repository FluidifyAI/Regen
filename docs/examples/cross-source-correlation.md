# Cross-Source Alert Correlation (OI-103)

## Overview

Cross-source alert correlation allows OpenIncident to group alerts from **different monitoring sources** (Prometheus, Grafana, CloudWatch, etc.) into a **single incident** when they share common labels.

This prevents alert fatigue when the same underlying issue triggers alerts across multiple monitoring systems.

---

## How It Works

### Without Cross-Source Correlation (Default Behavior)

By default, grouping rules use `match_labels` to determine both:
1. Which alerts the rule applies to
2. What makes alerts "the same group"

Example: Default rule `{"alertname": "*"}` groups alerts with the same `alertname`.

**Result:**
- Prometheus `HighCPU` alert → Incident 1
- Grafana `HighLatency` alert → Incident 2 (different incident)

Even if both alerts are for the same service, they create separate incidents because they have different alertnames.

---

### With Cross-Source Correlation

When you specify `cross_source_labels`, the grouping engine:
1. Uses `match_labels` to filter which alerts the rule applies to
2. Uses `cross_source_labels` to derive the group key (what makes alerts "the same")

Example: Rule with `cross_source_labels: ["service", "env"]`

**Result:**
- Prometheus `HighCPU` for `service=api, env=prod` → Incident 1
- Grafana `HighLatency` for `service=api, env=prod` → Links to Incident 1 ✅
- CloudWatch `HighErrorRate` for `service=api, env=prod` → Links to Incident 1 ✅

All three alerts create a **single incident** because they share `service=api, env=prod`.

---

## Example Use Cases

### 1. Service-Level Incidents

**Goal:** Group all alerts for the same service, regardless of source or alert type.

**Grouping Rule:**
```json
{
  "name": "Service-level incidents",
  "priority": 10,
  "enabled": true,
  "match_labels": {
    "service": "*"
  },
  "cross_source_labels": ["service", "env"],
  "time_window_seconds": 600
}
```

**Scenario:**
- Prometheus alerts: `HighCPU`, `HighMemory`
- Grafana alerts: `HighLatency`, `High5xxRate`
- CloudWatch alerts: `HighErrorCount`

All alerts for `service=api, env=production` → **1 incident**

---

### 2. Region-Wide Incidents

**Goal:** Group alerts from the same region across different services.

**Grouping Rule:**
```json
{
  "name": "Region-wide outages",
  "priority": 5,
  "enabled": true,
  "match_labels": {
    "severity": "critical"
  },
  "cross_source_labels": ["region"],
  "time_window_seconds": 300
}
```

**Scenario:**
- AWS CloudWatch: `HighAPILatency` in `us-east-1`
- Grafana: `DatabaseConnectionFailed` in `us-east-1`
- Prometheus: `HighCPU` in `us-east-1`

All critical alerts in `region=us-east-1` → **1 incident**

---

### 3. Critical Alerts Only

**Goal:** Only group critical alerts with the same service+env, ignore warnings.

**Grouping Rule:**
```json
{
  "name": "Critical service incidents",
  "priority": 20,
  "enabled": true,
  "match_labels": {
    "severity": "critical"
  },
  "cross_source_labels": ["service", "env"],
  "time_window_seconds": 600
}
```

**Scenario:**
- Prometheus critical `HighCPU` for `api/prod` → Incident 1
- Grafana critical `HighLatency` for `api/prod` → Links to Incident 1
- Prometheus warning `HighMemory` for `api/prod` → Separate incident (not critical)

---

## API Examples

### Create a Cross-Source Grouping Rule

```bash
curl -X POST http://localhost:8080/api/v1/grouping-rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Cross-source: Group by service and environment",
    "description": "Groups alerts from all monitoring sources for the same service in the same environment",
    "enabled": true,
    "priority": 50,
    "match_labels": {
      "severity": "critical"
    },
    "cross_source_labels": ["service", "env"],
    "time_window_seconds": 600
  }'
```

### Test Alert Grouping

Send alerts from different sources:

**Prometheus Alert:**
```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighCPU",
        "service": "api",
        "env": "production",
        "severity": "critical",
        "instance": "web-01"
      }
    }]
  }'
```

**Grafana Alert (5 seconds later):**
```bash
curl -X POST http://localhost:8080/api/v1/webhooks/grafana \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighLatency",
        "service": "api",
        "env": "production",
        "severity": "critical",
        "dashboard": "api-overview"
      }
    }]
  }'
```

**Expected Result:**
1. First alert creates Incident #1 with `group_key` derived from `service=api, env=production`
2. Second alert links to Incident #1 (same group key)
3. Slack shows both alerts in the same incident channel
4. Timeline shows: "Alert linked: HighLatency from Grafana"

---

## Database Schema

### Grouping Rule with Cross-Source Labels

```sql
INSERT INTO grouping_rules (
  name,
  description,
  priority,
  match_labels,
  cross_source_labels,
  time_window_seconds
) VALUES (
  'Cross-source: Service incidents',
  'Groups alerts from all monitoring sources for the same service',
  50,
  '{"service": "*"}',
  '["service", "env"]',  -- JSONB array
  600
);
```

### Query Incidents with Group Keys

```sql
-- Find all incidents created by grouping rule
SELECT
  incident_number,
  title,
  group_key,
  status,
  COUNT(alert_id) as alert_count
FROM incidents
LEFT JOIN incident_alerts ON incidents.id = incident_alerts.incident_id
WHERE group_key IS NOT NULL
GROUP BY incidents.id, incident_number, title, group_key, status
ORDER BY incident_number DESC;
```

---

## How Group Keys are Generated

### Algorithm

1. Extract label keys from `cross_source_labels` (or `match_labels` if empty)
2. Sort keys alphabetically for deterministic ordering
3. Build string: `"key1=value1|key2=value2|..."`
4. Hash with SHA256 for compact, consistent key

### Example

**Rule:** `cross_source_labels: ["service", "env"]`

**Alert Labels:**
```json
{
  "alertname": "HighCPU",
  "service": "api",
  "env": "production",
  "instance": "web-01"
}
```

**Group Key Derivation:**
1. Extract values: `service=api, env=production`
2. Sort and join: `"env=production|service=api"`
3. Hash (SHA256): `"67cc029136ef93ee8d95c02cf600dc4491d1b54af3330d7fb13145a25afe12fb"`

**Note:** `alertname` and `instance` are **not** included because they're not in `cross_source_labels`.

---

## Best Practices

### 1. Use Specific Labels for Cross-Source Correlation

✅ **Good:**
```json
{
  "cross_source_labels": ["service", "env", "cluster"]
}
```

❌ **Bad:**
```json
{
  "cross_source_labels": ["alertname"]
}
```

**Why:** `alertname` is source-specific. Use labels that are consistent across all monitoring sources.

---

### 2. Set Appropriate Time Windows

- **Short window (60-300s):** Rapid-fire alerts from the same incident
- **Medium window (300-600s):** Related alerts during an ongoing issue
- **Long window (1800-3600s):** Slow-developing issues

Example:
```json
{
  "name": "Rapid-fire service alerts",
  "cross_source_labels": ["service", "env"],
  "time_window_seconds": 120  // 2 minutes
}
```

---

### 3. Use Priority for Rule Ordering

Rules are evaluated in **priority order** (lowest number = highest priority).

Example:
```json
[
  {
    "priority": 10,
    "match_labels": {"severity": "critical", "service": "payment-api"},
    "cross_source_labels": ["service", "env", "region"]
  },
  {
    "priority": 50,
    "match_labels": {"severity": "critical"},
    "cross_source_labels": ["service", "env"]
  },
  {
    "priority": 100,
    "match_labels": {"alertname": "*"},
    "cross_source_labels": []  // Fallback to alertname grouping
  }
]
```

**First match wins!** High-priority rules are evaluated first.

---

### 4. Test Before Enabling

Create a rule with `enabled: false`, test with sample alerts, then enable:

```sql
-- Create disabled rule
INSERT INTO grouping_rules (..., enabled) VALUES (..., false);

-- Test with sample alerts
-- (check logs to see what group keys are generated)

-- Enable when satisfied
UPDATE grouping_rules SET enabled = true WHERE id = '...';
```

---

## Monitoring

### Check Grouping Effectiveness

```sql
-- How many alerts per incident?
SELECT
  group_key,
  COUNT(DISTINCT alert_id) as alert_count,
  COUNT(DISTINCT source) as source_count,
  array_agg(DISTINCT source) as sources
FROM incident_alerts
JOIN alerts ON alerts.id = incident_alerts.alert_id
JOIN incidents ON incidents.id = incident_alerts.incident_id
WHERE group_key IS NOT NULL
GROUP BY group_key
HAVING COUNT(DISTINCT source) > 1  -- Cross-source correlation happening
ORDER BY alert_count DESC
LIMIT 10;
```

### Identify Rules That Aren't Matching

```sql
-- Rules that haven't created any incidents
SELECT
  gr.name,
  gr.priority,
  gr.enabled,
  COUNT(i.id) as incident_count
FROM grouping_rules gr
LEFT JOIN incidents i ON i.group_key IS NOT NULL
WHERE gr.enabled = true
GROUP BY gr.id, gr.name, gr.priority, gr.enabled
HAVING COUNT(i.id) = 0;
```

---

## Troubleshooting

### Alerts Not Grouping Across Sources

**Problem:** Alerts from Prometheus and Grafana are creating separate incidents.

**Check:**
1. **Are label names consistent?**
   - Prometheus uses `service`, Grafana uses `service_name` → Won't group
   - Solution: Normalize labels at webhook ingestion

2. **Is the rule matching?**
   - Check `match_labels` filters
   - Run query: `SELECT * FROM grouping_rules WHERE enabled = true ORDER BY priority;`

3. **Is the time window too short?**
   - If alerts arrive 10 minutes apart but window is 5 minutes → Separate incidents
   - Solution: Increase `time_window_seconds`

4. **Are cross_source_labels set correctly?**
   - Check: `SELECT cross_source_labels FROM grouping_rules WHERE id = '...';`
   - Should be a JSONB array: `["service", "env"]`, not empty `[]`

---

### Rule Not Triggering

**Problem:** Rule exists but alerts still create separate incidents.

**Debug Steps:**

1. **Check rule priority:**
   ```sql
   SELECT priority, name, enabled
   FROM grouping_rules
   ORDER BY priority;
   ```
   Lower-priority rules might be matching first.

2. **Check rule cache:**
   - Rules are cached for 30 seconds
   - Restart backend or wait 30s after rule changes

3. **Check logs:**
   ```bash
   tail -f /var/log/openincident/backend.log | grep "grouping"
   ```

---

## Migration Guide

### Upgrading from v0.2 (No Grouping)

Existing incidents are unaffected. New grouping rules only apply to **new alerts**.

**Migration Steps:**

1. **Create default rule:**
   ```sql
   INSERT INTO grouping_rules (name, priority, match_labels, time_window_seconds)
   VALUES ('Default: group by alertname', 100, '{"alertname": "*"}', 300);
   ```

2. **Add cross-source rules gradually:**
   - Start with one service: `cross_source_labels: ["service", "env"]` for `service=api`
   - Monitor for 1-2 days
   - Expand to all services

3. **Disable old rules if needed:**
   ```sql
   UPDATE grouping_rules SET enabled = false WHERE name = 'Default: group by alertname';
   ```

---

## Related Documentation

- **[EPIC-013-PROGRESS.md](../EPIC-013-PROGRESS.md)** - Full implementation details
- **[ARCHITECTURE.md](../ARCHITECTURE.md)** - System design
- **[webhooks.md](webhooks.md)** - Webhook payload formats
- **Grouping Rules API** (OI-104) - Coming soon

---

**Status:** OI-103 ✅ Complete (v0.3)

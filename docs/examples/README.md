# OpenIncident Test Examples

This directory contains sample payloads and test scripts for validating your OpenIncident installation.

## Quick Start

### Option 1: Use the Test Script (Easiest)

```bash
# Make script executable
chmod +x test-alerts.sh

# Send all test alerts
./test-alerts.sh

# Send specific alert
./test-alerts.sh firing
./test-alerts.sh resolved
```

### Option 2: Use curl Commands Directly

```bash
# Set your OpenIncident URL
export OPENINCIDENT_URL="http://localhost:8080"

# Send firing alert
curl -X POST $OPENINCIDENT_URL/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-firing.json

# Send resolved alert
curl -X POST $OPENINCIDENT_URL/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-resolved.json
```

### Option 3: Copy-Paste Examples

See individual JSON files for complete payloads you can customize.

---

## Files

| File | Description |
|------|-------------|
| `alertmanager-firing.json` | Example of a critical alert in firing state |
| `alertmanager-resolved.json` | Example of the same alert transitioning to resolved |
| `alertmanager-warning.json` | Example of a warning-level alert |
| `alertmanager-info.json` | Example of an info-level alert (does not create incident) |
| `alertmanager-multiple.json` | Example with multiple alerts in one webhook |
| `test-alerts.sh` | Shell script to send all test alerts |

---

## What to Expect

### Firing Alert (`alertmanager-firing.json`)

**Expected behavior**:
1. Alert stored in database
2. Incident created (because severity is `critical`)
3. Slack channel created (if Slack configured): `#incident-001-high-error-rate`
4. Timeline entry: `incident_created`

**Verify**:
```bash
# Check incident was created
curl http://localhost:8080/api/v1/incidents

# Check alert was stored
curl http://localhost:8080/api/v1/alerts  # (returns 501 in v0.1, use incidents response)
```

### Resolved Alert (`alertmanager-resolved.json`)

**Expected behavior**:
1. Alert updated with `ended_at` timestamp
2. **Incident NOT automatically resolved** (must be done manually)

**Why doesn't resolved alert auto-resolve incident?**
- Incidents may have multiple linked alerts
- An incident represents a broader problem that may persist after one alert clears
- Responders should manually verify resolution before closing

**Manual resolution**:
```bash
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'
```

### Warning Alert (`alertmanager-warning.json`)

**Expected behavior**:
1. Alert stored in database
2. Incident created (because severity is `warning`)
3. Lower priority than critical alerts

### Info Alert (`alertmanager-info.json`)

**Expected behavior**:
1. Alert stored in database
2. **Incident NOT created** (info alerts are logged but don't trigger incidents)
3. Available for correlation with other alerts

### Multiple Alerts (`alertmanager-multiple.json`)

**Expected behavior**:
1. Each alert processed individually
2. Separate incidents created for each
3. Deduplication applied (same fingerprint = same alert)

---

## Testing Different Scenarios

### Scenario 1: Basic Alert Flow

```bash
# 1. Send firing alert
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-firing.json

# 2. Check incident created
curl http://localhost:8080/api/v1/incidents

# 3. Acknowledge incident
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"status": "acknowledged"}'

# 4. Send resolved alert
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-resolved.json

# 5. Manually resolve incident
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'
```

### Scenario 2: Deduplication

```bash
# Send same alert twice (same fingerprint)
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-firing.json

curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-firing.json

# Only ONE incident should be created
curl http://localhost:8080/api/v1/incidents | jq 'length'
# Expected: 1
```

### Scenario 3: Severity Levels

```bash
# Critical alert (creates incident)
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-firing.json

# Warning alert (creates incident)
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-warning.json

# Info alert (does NOT create incident)
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @alertmanager-info.json

# Check incidents (should be 2, not 3)
curl http://localhost:8080/api/v1/incidents | jq 'length'
# Expected: 2
```

---

## Customizing Payloads

To test with your own alert data:

1. **Copy an example file**
   ```bash
   cp alertmanager-firing.json my-custom-alert.json
   ```

2. **Edit the labels and annotations**
   ```json
   {
     "alerts": [{
       "labels": {
         "alertname": "MyCustomAlert",
         "severity": "critical",
         "service": "my-service",
         "environment": "production"
       },
       "annotations": {
         "summary": "My custom alert summary",
         "description": "Detailed description of the problem"
       }
     }]
   }
   ```

3. **Send it**
   ```bash
   curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
     -H "Content-Type: application/json" \
     -d @my-custom-alert.json
   ```

---

## Real Prometheus Integration

To receive alerts from your actual Prometheus/Alertmanager:

1. **Edit `alertmanager.yml`**:
   ```yaml
   receivers:
     - name: openincident
       webhook_configs:
         - url: http://localhost:8080/api/v1/webhooks/prometheus
           send_resolved: true
   ```

2. **Add route**:
   ```yaml
   route:
     receiver: openincident
     routes:
       - match:
           severity: critical
         receiver: openincident
       - match:
           severity: warning
         receiver: openincident
   ```

3. **Reload Alertmanager**:
   ```bash
   curl -X POST http://alertmanager:9093/-/reload
   ```

---

## Troubleshooting

### Alert sent but no incident created

**Check**:
1. Alert severity is `critical` or `warning` (not `info`)
2. Backend logs: `docker-compose logs backend | grep -i alert`
3. Health endpoint: `curl http://localhost:8080/ready`

### Slack channel not created

**Check**:
1. `SLACK_BOT_TOKEN` is set in `.env`
2. Bot has required OAuth scopes (see README.md)
3. Backend logs: `docker-compose logs backend | grep -i slack`

### Getting 500 errors

**Check**:
1. Database is running: `docker-compose ps db`
2. Redis is running: `docker-compose ps redis`
3. Migrations ran: `docker-compose logs backend | grep migration`

---

## Additional Resources

- **API Documentation**: [docs/API.md](../API.md)
- **Setup Guide**: [README.md](../../README.md)
- **Troubleshooting**: [README.md#troubleshooting](../../README.md#troubleshooting)

---

**Need help?** Open an issue on GitHub or ask in Discussions.

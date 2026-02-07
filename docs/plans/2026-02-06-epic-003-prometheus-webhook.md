# Epic 003: Prometheus Webhook Ingestion - Implementation Plan

## Context

This epic implements the first production webhook endpoint that receives Prometheus Alertmanager webhooks, parses alerts, stores them with deduplication, and auto-creates incidents for critical/warning severity alerts. This is the core value delivery for v0.1 - enabling users to receive alerts and automatically create incidents.

**Why this change:**
- Delivers on the CLAUDE.md v0.1 definition of done: "Alert fires → incident auto-created → Slack channel appears"
- Builds on the foundation from Epic 001 (infrastructure) and Epic 002 (data models/repositories)
- Enables the full webhook → alert → incident → timeline flow

**User Story:**
As a DevOps engineer, I want to point my Prometheus Alertmanager at OpenIncident's webhook endpoint so that critical and warning alerts automatically create incidents with full alert context, without manual intervention.

---

## Architectural Decisions

### 1. Service Layer Architecture

**Two-service approach** (`AlertService` + `IncidentService`):
- **AlertService**: Alert parsing, normalization, deduplication (reusable for Grafana, CloudWatch webhooks later)
- **IncidentService**: Incident creation, alert linking, timeline management (will grow to handle manual incident creation, Slack integration)

**Why not monolithic:** Clear separation of concerns, reusability, testability

### 2. Transaction Boundary

Single transaction for atomic operations:
```
BEGIN TRANSACTION
  1. Create/Update Alert
  2. Create Incident (if critical/warning)
  3. Link Alert to Incident (incident_alerts table)
  4. Create Timeline Entry (type=incident_created)
COMMIT
```

**Why:** Ensures data consistency - if any step fails, everything rolls back. Critical for audit trail integrity.

### 3. Processing Model

**Synchronous processing** (no Redis queue for v0.1):
- Simpler implementation and error handling
- Database operations complete in <100ms
- Webhook returns success only after DB commit
- Future: Add async queue in v0.3 for alert grouping rules

### 4. Deduplication Strategy

Use `AlertRepository.GetByExternalID(source, external_id)`:
- Fingerprint from Alertmanager = external_id
- Unique constraint on (source, external_id) prevents race conditions
- Update only: status, title, description, ended_at
- **received_at remains immutable** (first time alert was seen)

### 5. Metrics (OI-020)

**Defer Prometheus metrics to future task:**
- For now: structured logging captures all needed data (timestamp, count, duration, incidents_created)
- Why defer: Prometheus client_golang adds complexity, not critical for MVP
- Logs can be parsed for metrics post-launch

---

## Implementation Steps

### Task 1: Define Alertmanager Webhook Payload Structs (OI-015)

**Create:** `backend/internal/models/webhooks/prometheus.go`

Define structs matching Alertmanager webhook JSON format:
- `AlertmanagerPayload` - Top-level webhook payload
- `AlertmanagerAlert` - Individual alert within payload

**Key fields:**
- JSON tags matching Alertmanager exactly
- `time.Time` for startsAt/endsAt (ISO8601 parsing)
- Fingerprint string for deduplication

**Verification:** Compile, test with sample Alertmanager JSON

---

### Task 2: Implement Alert Service (OI-017 + OI-018)

**Create:** `backend/internal/services/alert_service.go`

**Interface:**
```go
type AlertService interface {
    ProcessAlertmanagerPayload(payload *webhooks.AlertmanagerPayload) (*ProcessingResult, error)
}

type ProcessingResult struct {
    Received         int
    Created          int
    Updated          int
    IncidentsCreated int
}
```

**Key methods:**

1. **ProcessAlertmanagerPayload** - Orchestrates processing of all alerts in webhook
   - Loop through payload.Alerts
   - For each: normalize → create/update → check for incident creation
   - Track counts for response

2. **normalizeAlert** (private) - Maps Alertmanager alert to internal Alert model
   - Extract title from `labels["alertname"]`
   - Extract description from `annotations["summary"]` or `annotations["description"]`
   - Parse severity from `labels["severity"]` (default: "warning" if missing)
   - Store full labels/annotations as JSONB
   - Store raw payload as JSONB
   - Use fingerprint as external_id
   - Set source = "prometheus"

3. **CreateOrUpdateAlert** (private) - Deduplication logic
   - Try `GetByExternalID("prometheus", fingerprint)`
   - If NotFoundError: Create new alert
   - If found: Update mutable fields (status, title, description, ended_at)
   - Return whether created or updated

**Dependencies:** Inject `AlertRepository`, `IncidentService`

---

### Task 3: Implement Incident Service (OI-019)

**Create:** `backend/internal/services/incident_service.go`

**Interface:**
```go
type IncidentService interface {
    CreateIncidentFromAlert(alert *models.Alert) (*models.Incident, error)
    ShouldCreateIncident(severity models.AlertSeverity) bool
}
```

**Key methods:**

1. **ShouldCreateIncident** - Determines if alert triggers incident
   - `critical` → YES
   - `warning` → YES
   - `info` → NO

2. **CreateIncidentFromAlert** - Creates incident with alert context
   - Title from alert.Title
   - Slug from `generateSlug(title)` - URL-safe, max 50 chars, lowercase, hyphens
   - Status: "triggered"
   - Severity mapping: critical→critical, warning→high
   - CreatedByType: "system", CreatedByID: "alertmanager"

3. **CreateIncidentWithAlert** (private) - Atomic transaction
   ```go
   db.Transaction(func(tx *gorm.DB) error {
       // 1. Create incident
       incidentRepo.Create(incident)

       // 2. Link alert
       incidentRepo.LinkAlert(incident.ID, alert.ID, "system", "alertmanager")

       // 3. Create timeline entry
       timelineRepo.Create(&TimelineEntry{
           IncidentID: incident.ID,
           Type: "incident_created",
           ActorType: "system",
           ActorID: "alertmanager",
           Content: {"trigger": "alert", "alert_id": alert.ID, "source": "prometheus"}
       })
   })
   ```

**Dependencies:** Inject `IncidentRepository`, `TimelineRepository`, `*gorm.DB` (for transactions)

---

### Task 4: Implement Prometheus Webhook Handler (OI-016)

**Create:** `backend/internal/api/handlers/prometheus_webhook.go`

**Handler factory:**
```go
func PrometheusWebhook(alertSvc services.AlertService) gin.HandlerFunc {
    return func(c *gin.Context) {
        startTime := time.Now()

        // 1. Parse JSON payload
        var payload webhooks.AlertmanagerPayload
        if err := c.ShouldBindJSON(&payload); err != nil {
            c.JSON(400, gin.H{"error": "invalid payload"})
            return
        }

        // 2. Validate (basic checks: alerts array exists)
        if len(payload.Alerts) == 0 {
            c.JSON(400, gin.H{"error": "no alerts in payload"})
            return
        }

        // 3. Process alerts
        result, err := alertSvc.ProcessAlertmanagerPayload(&payload)
        if err != nil {
            slog.Error("webhook processing failed", "error", err)
            c.JSON(500, gin.H{"error": "internal server error"})
            return
        }

        // 4. Log metrics (OI-020)
        duration := time.Since(startTime)
        slog.Info("webhook processed",
            "source", "prometheus",
            "received", result.Received,
            "created", result.Created,
            "updated", result.Updated,
            "incidents_created", result.IncidentsCreated,
            "duration_ms", duration.Milliseconds(),
        )

        // 5. Return success response
        c.JSON(200, gin.H{
            "received": result.Received,
            "incidents_created": result.IncidentsCreated,
        })
    }
}
```

**Error handling:**
- 400: Malformed JSON, missing fields, validation errors
- 500: Database errors, service errors

---

### Task 5: Wire Up Route Registration

**Modify:** `backend/internal/api/routes.go`

Changes:
1. Import services package
2. Initialize services at top of `SetupRoutes()`:
   ```go
   // Initialize repositories
   alertRepo := repository.NewAlertRepository(db)
   incidentRepo := repository.NewIncidentRepository(db)
   timelineRepo := repository.NewTimelineRepository(db)

   // Initialize services
   incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo, db)
   alertSvc := services.NewAlertService(alertRepo, incidentSvc)
   ```

3. Replace stub handler with real handler:
   ```go
   webhooks.POST("/prometheus", handlers.PrometheusWebhook(alertSvc))
   ```

4. Remove the stub inline function

---

### Task 6: Add Structured Logging (OI-020)

**Already handled in webhook handler** (Task 4), but verify:

Log entry includes:
- ✅ Timestamp (automatic with slog)
- ✅ Source ("prometheus")
- ✅ Alert count (received)
- ✅ Processing time (duration_ms)
- ✅ Incidents created

**Note:** Prometheus metrics (counters, histograms) deferred to separate future task

---

## Critical Files

| File | Action | Purpose |
|------|--------|---------|
| `backend/internal/models/webhooks/prometheus.go` | Create | Alertmanager webhook payload structs |
| `backend/internal/services/alert_service.go` | Create | Alert parsing, normalization, deduplication |
| `backend/internal/services/incident_service.go` | Create | Incident creation with transactional alert linking |
| `backend/internal/api/handlers/prometheus_webhook.go` | Create | HTTP webhook endpoint handler |
| `backend/internal/api/routes.go` | Modify | Service initialization and route registration |

---

## Verification Steps

### 1. Unit Tests
```bash
go test ./internal/services/... -v
```
- Test alert normalization (various severity values, missing fields)
- Test deduplication (create vs update paths)
- Test incident creation logic
- Test slug generation edge cases

### 2. Integration Tests
```bash
go test ./internal/api/handlers/... -v
```
- Test complete webhook flow with database
- Test error cases (malformed payload, validation errors)
- Test response format

### 3. Manual Testing
```bash
# Start services
make dev

# Send test webhook (firing alert)
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "version": "4",
    "status": "firing",
    "alerts": [
      {
        "status": "firing",
        "labels": {
          "alertname": "HighCPUUsage",
          "severity": "critical",
          "instance": "web-01"
        },
        "annotations": {
          "summary": "CPU usage above 90%"
        },
        "startsAt": "2026-02-06T15:30:00Z",
        "endsAt": "0001-01-01T00:00:00Z",
        "fingerprint": "abc123"
      }
    ]
  }'

# Expected response: {"received": 1, "incidents_created": 1}
```

### 4. Database Verification
```sql
-- Check alert was created
SELECT id, source, external_id, status, severity, title
FROM alerts
ORDER BY received_at DESC LIMIT 1;

-- Check incident was created
SELECT id, incident_number, title, status, severity
FROM incidents
ORDER BY triggered_at DESC LIMIT 1;

-- Check alert is linked to incident
SELECT * FROM incident_alerts ORDER BY linked_at DESC LIMIT 1;

-- Check timeline entry was created
SELECT incident_id, type, actor_type, content
FROM timeline_entries
ORDER BY timestamp DESC LIMIT 1;
```

### 5. Log Verification
Check logs contain structured data:
```json
{
  "time": "2026-02-06T15:30:45Z",
  "level": "INFO",
  "msg": "webhook processed",
  "source": "prometheus",
  "received": 1,
  "created": 1,
  "updated": 0,
  "incidents_created": 1,
  "duration_ms": 45
}
```

---

## Implementation Order

1. **OI-015**: Webhook payload structs (foundation)
2. **OI-017 + OI-018**: Alert service (core business logic)
3. **OI-019**: Incident service (incident creation)
4. **OI-016**: Webhook handler (HTTP layer)
5. **Integration**: Route registration
6. **OI-020**: Verify logging (already included in handler)
7. **Testing**: Unit, integration, manual tests

---

## Success Criteria

✅ Alertmanager can POST to `/api/v1/webhooks/prometheus`
✅ Alerts are stored with proper parsing (labels, annotations, severity)
✅ Critical/warning alerts auto-create incidents
✅ Incidents are linked to alerts via `incident_alerts` table
✅ Timeline entries created with `type=incident_created`
✅ Duplicate alerts (same fingerprint) update existing record
✅ Info severity alerts do NOT create incidents
✅ Response includes `{received: N, incidents_created: M}`
✅ Malformed payloads return 400 with error message
✅ All operations are transactional (atomic)
✅ Structured logging captures all metrics

---

## Risk Mitigation

**Transaction failures:** All DB operations in single transaction - ensures consistency

**Duplicate alerts:** Unique constraint on (source, external_id) prevents races

**Service errors:** Custom error types allow handlers to map to correct HTTP status codes

**Rollback:** No migrations needed, can disable route by commenting out registration

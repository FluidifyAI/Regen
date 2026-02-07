# OpenIncident API Documentation

**Version**: v0.1
**Base URL**: `http://localhost:8080` (default)
**API Prefix**: `/api/v1`

---

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Response Format](#response-format)
- [Error Handling](#error-handling)
- [Health & Metrics](#health--metrics)
- [Webhooks](#webhooks)
  - [Prometheus Alertmanager](#post-apiv1webhooksprometheus)
- [Incidents](#incidents)
  - [List Incidents](#get-apiv1incidents)
  - [Get Incident](#get-apiv1incidentsid)
  - [Create Incident](#post-apiv1incidents)
  - [Update Incident](#patch-apiv1incidentsid)
  - [Get Incident Timeline](#get-apiv1incidentsidtimeline)
  - [Add Timeline Entry](#post-apiv1incidentsidtimeline)
- [Data Models](#data-models)
- [Examples](#examples)

---

## Overview

The OpenIncident API is a RESTful JSON API for managing incidents, alerts, and on-call workflows.

**Key Features**:
- Alert ingestion from Prometheus, Grafana, and other monitoring systems
- Incident lifecycle management (triggered → acknowledged → resolved)
- Immutable timeline for audit trail
- Slack integration for incident channels
- Prometheus metrics exposure

**API Characteristics**:
- All timestamps are in ISO 8601 format (UTC)
- All IDs are UUIDs (except incident numbers which are auto-incremented integers)
- All request/response bodies use JSON
- Idempotent webhook processing (duplicate alerts deduplicated)
- CORS enabled for cross-origin requests

---

## Authentication

**v0.1**: No authentication required. All endpoints are public.

**v0.2+**: API key authentication will be required for non-webhook endpoints.

Future authentication will use Bearer tokens:
```http
Authorization: Bearer <api-key>
```

---

## Response Format

### Success Responses

Single resource:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "Database connection timeout",
  "status": "triggered",
  "severity": "critical",
  ...
}
```

Collection:
```json
{
  "data": [ ... ],
  "meta": {
    "total": 42,
    "page": 1,
    "per_page": 20
  }
}
```

**Note**: In v0.1, list endpoints return simple arrays. Pagination will be added in v0.2.

---

## Error Handling

### Error Response Format

```json
{
  "error": "Error message description",
  "code": "ERROR_CODE",
  "details": { ... }  // Optional, context-specific
}
```

### HTTP Status Codes

| Code | Meaning | Usage |
|------|---------|-------|
| `200` | OK | Successful GET, PATCH |
| `201` | Created | Successful POST (resource created) |
| `400` | Bad Request | Invalid request body or parameters |
| `404` | Not Found | Resource does not exist |
| `422` | Unprocessable Entity | Validation error |
| `500` | Internal Server Error | Server-side error |
| `503` | Service Unavailable | Database or Redis unhealthy |

### Common Error Codes

- `VALIDATION_ERROR` — Request validation failed
- `NOT_FOUND` — Resource not found
- `DUPLICATE` — Resource already exists
- `INTERNAL_ERROR` — Unexpected server error

---

## Health & Metrics

### `GET /health`

Simple health check. Always returns `200 OK` if the server is running.

**Response**:
```json
{
  "status": "ok"
}
```

---

### `GET /ready`

Readiness check for Kubernetes. Verifies database and Redis connectivity.

**Response (Healthy)**:
```json
{
  "status": "ready",
  "database": "ok",
  "redis": "ok"
}
```

**Response (Unhealthy)** — HTTP `503`:
```json
{
  "status": "not ready",
  "database": "error",
  "redis": "ok"
}
```

---

### `GET /metrics`

Prometheus metrics endpoint.

**Response**: Prometheus text format

**Available Metrics**:
- `http_requests_total` — Total HTTP requests (method, path, status)
- `http_request_duration_seconds` — Request latency histogram
- `incidents_total` — Total incidents by status
- `incidents_by_severity` — Total incidents by severity
- `alerts_total` — Total alerts by status
- `db_connections_open` — Open database connections
- `db_connections_in_use` — Database connections in use
- `db_connections_idle` — Idle database connections

---

## Webhooks

### `POST /api/v1/webhooks/prometheus`

Receives alerts from Prometheus Alertmanager.

**Request Headers**:
```http
Content-Type: application/json
```

**Request Body** (Alertmanager format):
```json
{
  "receiver": "openincident",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighErrorRate",
        "severity": "critical",
        "service": "api",
        "environment": "production"
      },
      "annotations": {
        "summary": "Error rate above 5%",
        "description": "The API service is experiencing elevated error rates",
        "runbook_url": "https://wiki.example.com/runbooks/high-error-rate"
      },
      "startsAt": "2024-01-15T10:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus:9090/graph?g0.expr=...",
      "fingerprint": "1234567890abcdef"
    }
  ],
  "groupLabels": {
    "alertname": "HighErrorRate"
  },
  "commonLabels": {
    "severity": "critical"
  },
  "commonAnnotations": {},
  "externalURL": "http://alertmanager:9093",
  "version": "4",
  "groupKey": "{}:{alertname=\"HighErrorRate\"}"
}
```

**Response** — HTTP `201`:
```json
{
  "received": 1,
  "processed": 1,
  "incidents_created": 1,
  "incidents_updated": 0
}
```

**Behavior**:
1. Each alert is stored in the `alerts` table
2. Alerts with `severity: critical` or `severity: warning` **automatically create incidents**
3. Alerts with `severity: info` are stored but do not create incidents
4. Duplicate alerts (same fingerprint) update existing alerts
5. If Slack is configured, incidents create channels like `#incident-001-high-error-rate`
6. Resolved alerts (`status: resolved`) update the `ended_at` timestamp

**Deduplication**:
- Alerts are deduplicated by `fingerprint` (provided by Alertmanager)
- If no fingerprint, deduplication uses `alertname` + `labels` hash

---

### `POST /api/v1/webhooks/grafana`

**Status**: Not implemented (returns `501 Not Implemented`)

Planned for v0.3.

---

## Incidents

### `GET /api/v1/incidents`

List all incidents, ordered by most recent first.

**Query Parameters**:
- `status` (optional) — Filter by status: `triggered`, `acknowledged`, `resolved`, `canceled`
- `severity` (optional) — Filter by severity: `critical`, `high`, `medium`, `low`

**Example**:
```http
GET /api/v1/incidents?status=triggered&severity=critical
```

**Response** — HTTP `200`:
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "incident_number": 1,
    "title": "High error rate on API service",
    "slug": "high-error-rate-on-api-service",
    "status": "triggered",
    "severity": "critical",
    "summary": "Error rate exceeded 5% threshold",
    "slack_channel_id": "C01ABC123XY",
    "slack_channel_name": "#incident-001-high-error-rate",
    "created_at": "2024-01-15T10:00:00Z",
    "triggered_at": "2024-01-15T10:00:00Z",
    "acknowledged_at": null,
    "resolved_at": null,
    "created_by_type": "system",
    "created_by_id": "prometheus-webhook",
    "commander_id": null,
    "labels": {},
    "custom_fields": {}
  }
]
```

---

### `GET /api/v1/incidents/:id`

Get a single incident by ID or incident number.

**Path Parameters**:
- `id` — UUID or incident number (e.g., `550e8400-e29b-41d4-a716-446655440000` or `1`)

**Example**:
```http
GET /api/v1/incidents/1
```

**Response** — HTTP `200`:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "incident_number": 1,
  "title": "High error rate on API service",
  "slug": "high-error-rate-on-api-service",
  "status": "triggered",
  "severity": "critical",
  "summary": "Error rate exceeded 5% threshold",
  "slack_channel_id": "C01ABC123XY",
  "slack_channel_name": "#incident-001-high-error-rate",
  "created_at": "2024-01-15T10:00:00Z",
  "triggered_at": "2024-01-15T10:00:00Z",
  "acknowledged_at": null,
  "resolved_at": null,
  "created_by_type": "system",
  "created_by_id": "prometheus-webhook",
  "commander_id": null,
  "labels": {},
  "custom_fields": {},
  "alerts": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "external_id": "prom:HighErrorRate:1234567890",
      "source": "prometheus",
      "status": "firing",
      "severity": "critical",
      "title": "HighErrorRate",
      "description": "Error rate above 5%",
      "labels": {
        "alertname": "HighErrorRate",
        "severity": "critical",
        "service": "api"
      },
      "annotations": {
        "summary": "Error rate above 5%"
      },
      "started_at": "2024-01-15T10:00:00Z",
      "received_at": "2024-01-15T10:00:05Z"
    }
  ]
}
```

**Error** — HTTP `404`:
```json
{
  "error": "incident not found"
}
```

---

### `POST /api/v1/incidents`

Manually create a new incident.

**Request Body**:
```json
{
  "title": "Database connection timeout",
  "severity": "high",
  "summary": "Multiple services reporting DB connection timeouts"
}
```

**Fields**:
- `title` (required) — String, max 500 characters
- `severity` (required) — One of: `critical`, `high`, `medium`, `low`
- `summary` (optional) — String, detailed description
- `labels` (optional) — Object, custom key-value pairs
- `custom_fields` (optional) — Object, custom metadata

**Response** — HTTP `201`:
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "incident_number": 2,
  "title": "Database connection timeout",
  "slug": "database-connection-timeout",
  "status": "triggered",
  "severity": "high",
  "summary": "Multiple services reporting DB connection timeouts",
  "slack_channel_id": "C02DEF456ZA",
  "slack_channel_name": "#incident-002-database-connection-timeout",
  "created_at": "2024-01-15T11:00:00Z",
  "triggered_at": "2024-01-15T11:00:00Z",
  "created_by_type": "user",
  "created_by_id": "",
  "commander_id": null,
  "labels": {},
  "custom_fields": {}
}
```

**Validation Error** — HTTP `400`:
```json
{
  "error": "validation error",
  "details": {
    "title": "required",
    "severity": "must be one of: critical, high, medium, low"
  }
}
```

---

### `PATCH /api/v1/incidents/:id`

Update an incident (status, severity, summary, etc.).

**Path Parameters**:
- `id` — UUID or incident number

**Request Body** (all fields optional):
```json
{
  "status": "acknowledged",
  "severity": "high",
  "summary": "Database issue identified - restarting replicas",
  "commander_id": "880e8400-e29b-41d4-a716-446655440003"
}
```

**Allowed Status Transitions**:
- `triggered` → `acknowledged`, `resolved`, `canceled`
- `acknowledged` → `resolved`, `canceled`
- `resolved` → (final state, cannot transition)
- `canceled` → (final state, cannot transition)

**Response** — HTTP `200`:
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "incident_number": 2,
  "status": "acknowledged",
  "acknowledged_at": "2024-01-15T11:05:00Z",
  ...
}
```

**Behavior**:
- Changing `status` updates corresponding timestamp (`acknowledged_at`, `resolved_at`)
- Timeline entry is automatically created for status/severity changes
- Slack channel receives update notification (if configured)

---

### `GET /api/v1/incidents/:id/timeline`

Get the timeline for an incident (immutable audit log).

**Path Parameters**:
- `id` — UUID or incident number

**Example**:
```http
GET /api/v1/incidents/1/timeline
```

**Response** — HTTP `200`:
```json
[
  {
    "id": "990e8400-e29b-41d4-a716-446655440004",
    "incident_id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2024-01-15T10:00:00Z",
    "type": "incident_created",
    "actor_type": "system",
    "actor_id": "prometheus-webhook",
    "content": {
      "title": "High error rate on API service",
      "severity": "critical"
    },
    "created_at": "2024-01-15T10:00:00Z"
  },
  {
    "id": "aa0e8400-e29b-41d4-a716-446655440005",
    "incident_id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2024-01-15T10:05:00Z",
    "type": "status_changed",
    "actor_type": "user",
    "actor_id": "user-123",
    "content": {
      "from": "triggered",
      "to": "acknowledged"
    },
    "created_at": "2024-01-15T10:05:00Z"
  }
]
```

**Timeline Entry Types**:
- `incident_created` — Incident was created
- `status_changed` — Status changed (triggered → acknowledged → resolved)
- `severity_changed` — Severity changed
- `alert_linked` — Alert was linked to incident
- `message` — User added a comment/message
- `responder_added` — Responder was added to incident
- `escalated` — Incident was escalated
- `summary_generated` — AI summary was generated (future)

---

### `POST /api/v1/incidents/:id/timeline`

Add a timeline entry (message) to an incident.

**Path Parameters**:
- `id` — UUID or incident number

**Request Body**:
```json
{
  "type": "message",
  "content": {
    "message": "Restarted database replicas, monitoring for recovery"
  }
}
```

**Response** — HTTP `201`:
```json
{
  "id": "bb0e8400-e29b-41d4-a716-446655440006",
  "incident_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T10:10:00Z",
  "type": "message",
  "actor_type": "user",
  "actor_id": "",
  "content": {
    "message": "Restarted database replicas, monitoring for recovery"
  },
  "created_at": "2024-01-15T10:10:00Z"
}
```

**Note**: Timeline entries are **immutable**. They cannot be updated or deleted.

---

## Data Models

### Incident

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique identifier |
| `incident_number` | Integer | Human-friendly number (e.g., 1, 2, 3...) |
| `title` | String | Incident title (max 500 chars) |
| `slug` | String | URL-friendly slug |
| `status` | String | `triggered`, `acknowledged`, `resolved`, `canceled` |
| `severity` | String | `critical`, `high`, `medium`, `low` |
| `summary` | String | Detailed description |
| `slack_channel_id` | String | Slack channel ID (e.g., C01ABC123XY) |
| `slack_channel_name` | String | Slack channel name (e.g., #incident-001-...) |
| `created_at` | Timestamp | When incident was created (immutable) |
| `triggered_at` | Timestamp | When incident was triggered (immutable) |
| `acknowledged_at` | Timestamp | When incident was acknowledged |
| `resolved_at` | Timestamp | When incident was resolved |
| `created_by_type` | String | `system` or `user` |
| `created_by_id` | String | User ID or system identifier |
| `commander_id` | UUID | Incident commander user ID |
| `labels` | Object | Custom labels (JSONB) |
| `custom_fields` | Object | Custom metadata (JSONB) |
| `alerts` | Array | Related alerts (when included) |

---

### Alert

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique identifier |
| `external_id` | String | External identifier from source |
| `source` | String | Source system (e.g., `prometheus`) |
| `fingerprint` | String | Deduplication fingerprint |
| `status` | String | `firing` or `resolved` |
| `severity` | String | `critical`, `warning`, `info` |
| `title` | String | Alert title |
| `description` | String | Alert description |
| `labels` | Object | Alert labels (JSONB) |
| `annotations` | Object | Alert annotations (JSONB) |
| `raw_payload` | Object | Original webhook payload (JSONB) |
| `started_at` | Timestamp | When alert started firing |
| `ended_at` | Timestamp | When alert was resolved |
| `received_at` | Timestamp | When webhook was received (immutable) |
| `created_at` | Timestamp | Database creation timestamp |

---

### Timeline Entry

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique identifier |
| `incident_id` | UUID | Related incident |
| `timestamp` | Timestamp | When event occurred (immutable) |
| `type` | String | Event type (see types above) |
| `actor_type` | String | `user`, `system`, `slack_bot` |
| `actor_id` | String | User ID or system identifier |
| `content` | Object | Event-specific data (JSONB) |
| `created_at` | Timestamp | Database creation timestamp |

---

## Examples

### Complete Incident Lifecycle

#### 1. Alert Fires

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "openincident",
    "status": "firing",
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "DatabaseDown",
        "severity": "critical",
        "database": "users-db"
      },
      "annotations": {
        "summary": "Database users-db is not responding",
        "description": "Database has been down for 2 minutes"
      },
      "startsAt": "2024-01-15T12:00:00Z"
    }]
  }'
```

**Result**: Incident #1 created, Slack channel `#incident-001-database-down` created

#### 2. Acknowledge Incident

```bash
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{
    "status": "acknowledged",
    "summary": "Database team investigating connection pool exhaustion"
  }'
```

**Result**: Timeline entry created, Slack channel updated with acknowledgment

#### 3. Add Timeline Message

```bash
curl -X POST http://localhost:8080/api/v1/incidents/1/timeline \
  -H "Content-Type: application/json" \
  -d '{
    "type": "message",
    "content": {
      "message": "Root cause identified: connection pool misconfigured. Deploying fix."
    }
  }'
```

**Result**: Message added to timeline and posted to Slack

#### 4. Resolve Incident

```bash
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{
    "status": "resolved",
    "summary": "Fix deployed. Database connection pool increased. Monitoring for 30 minutes."
  }'
```

**Result**: Incident resolved, `resolved_at` timestamp set, Slack channel archived (future)

#### 5. Alert Resolves

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "openincident",
    "status": "resolved",
    "alerts": [{
      "status": "resolved",
      "labels": {
        "alertname": "DatabaseDown",
        "severity": "critical",
        "database": "users-db"
      },
      "annotations": {
        "summary": "Database users-db is not responding"
      },
      "startsAt": "2024-01-15T12:00:00Z",
      "endsAt": "2024-01-15T12:15:00Z"
    }]
  }'
```

**Result**: Alert updated with `ended_at` timestamp

---

### Testing with curl

#### Send a Test Alert (Firing)

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "test",
    "status": "firing",
    "alerts": [{
      "status": "firing",
      "labels": {"alertname": "TestAlert", "severity": "critical"},
      "annotations": {"summary": "Test alert for validation"},
      "startsAt": "2024-01-01T00:00:00Z"
    }]
  }'
```

#### Send a Test Alert (Resolved)

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "test",
    "status": "resolved",
    "alerts": [{
      "status": "resolved",
      "labels": {"alertname": "TestAlert", "severity": "critical"},
      "annotations": {"summary": "Test alert for validation"},
      "startsAt": "2024-01-01T00:00:00Z",
      "endsAt": "2024-01-01T00:05:00Z"
    }]
  }'
```

#### List All Incidents

```bash
curl http://localhost:8080/api/v1/incidents
```

#### Get Incident by Number

```bash
curl http://localhost:8080/api/v1/incidents/1
```

#### Filter Triggered Critical Incidents

```bash
curl 'http://localhost:8080/api/v1/incidents?status=triggered&severity=critical'
```

---

## Changelog

### v0.1 (Current)

- ✅ Health and readiness endpoints
- ✅ Prometheus webhook ingestion
- ✅ Incident CRUD operations
- ✅ Timeline entries
- ✅ Slack channel creation
- ✅ Prometheus metrics exposure

### v0.2 (Planned)

- [ ] API key authentication
- [ ] Pagination for list endpoints
- [ ] Filtering and sorting
- [ ] User management
- [ ] Incident assignment

### v0.3 (Planned)

- [ ] Grafana webhook
- [ ] CloudWatch webhook
- [ ] Generic webhook
- [ ] Alert routing rules
- [ ] Alert grouping

---

## Support

- **API Issues**: [GitHub Issues](https://github.com/yourusername/openincident/issues)
- **Community**: [GitHub Discussions](https://github.com/yourusername/openincident/discussions)
- **Documentation**: [README.md](../README.md)

---

*Last updated: 2024-01-15*

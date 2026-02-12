# Grouping Rules CRUD API (OI-104)

## Overview

The Grouping Rules API allows you to manage alert grouping rules via REST endpoints. This enables dynamic configuration of how alerts are grouped into incidents without requiring database access or application restarts.

**Base URL:** `/api/v1/grouping-rules`

---

## API Endpoints

### List Grouping Rules

**GET** `/api/v1/grouping-rules`

Returns all grouping rules, optionally filtered by enabled status.

**Query Parameters:**
- `enabled` (optional): Filter by enabled status (`true`, `false`)

**Example Request:**
```bash
# Get all rules
curl http://localhost:8080/api/v1/grouping-rules

# Get only enabled rules
curl http://localhost:8080/api/v1/grouping-rules?enabled=true

# Get only disabled rules
curl http://localhost:8080/api/v1/grouping-rules?enabled=false
```

**Example Response:**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Cross-source: Group by service and env",
      "description": "Groups alerts from all sources for the same service/env",
      "enabled": true,
      "priority": 50,
      "match_labels": {
        "severity": "critical"
      },
      "cross_source_labels": ["service", "env"],
      "time_window_seconds": 600,
      "created_at": "2026-02-12T10:00:00Z",
      "updated_at": "2026-02-12T10:00:00Z"
    }
  ],
  "total": 1
}
```

---

### Get Grouping Rule

**GET** `/api/v1/grouping-rules/:id`

Returns a single grouping rule by ID.

**Path Parameters:**
- `id` (required): UUID of the grouping rule

**Example Request:**
```bash
curl http://localhost:8080/api/v1/grouping-rules/550e8400-e29b-41d4-a716-446655440000
```

**Example Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Cross-source: Group by service and env",
  "description": "Groups alerts from all sources for the same service/env",
  "enabled": true,
  "priority": 50,
  "match_labels": {
    "severity": "critical"
  },
  "cross_source_labels": ["service", "env"],
  "time_window_seconds": 600,
  "created_at": "2026-02-12T10:00:00Z",
  "updated_at": "2026-02-12T10:00:00Z"
}
```

**Error Responses:**
- `404 Not Found` - Rule with specified ID does not exist
- `400 Bad Request` - Invalid UUID format

---

### Create Grouping Rule

**POST** `/api/v1/grouping-rules`

Creates a new grouping rule.

**Request Body:**
```json
{
  "name": "Cross-source: Group by service and env",
  "description": "Groups alerts from all sources for the same service/env",
  "enabled": true,
  "priority": 50,
  "match_labels": {
    "severity": "critical"
  },
  "cross_source_labels": ["service", "env"],
  "time_window_seconds": 600
}
```

**Field Validations:**
- `name` (required): 1-255 characters
- `description` (optional): Max 1000 characters
- `enabled` (optional): Boolean, defaults to `true`
- `priority` (required): Integer 1-1000, must be unique
- `match_labels` (required): JSON object (cannot be empty)
- `cross_source_labels` (optional): Array of strings
- `time_window_seconds` (required): Integer 1-86400 (1 second to 24 hours)

**Example Request:**
```bash
curl -X POST http://localhost:8080/api/v1/grouping-rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Critical service incidents",
    "description": "Groups critical alerts from the same service",
    "enabled": true,
    "priority": 10,
    "match_labels": {
      "severity": "critical"
    },
    "cross_source_labels": ["service", "env"],
    "time_window_seconds": 600
  }'
```

**Success Response (201 Created):**
```json
{
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Critical service incidents",
  "description": "Groups critical alerts from the same service",
  "enabled": true,
  "priority": 10,
  "match_labels": {
    "severity": "critical"
  },
  "cross_source_labels": ["service", "env"],
  "time_window_seconds": 600,
  "created_at": "2026-02-12T15:30:00Z",
  "updated_at": "2026-02-12T15:30:00Z"
}
```

**Error Responses:**
- `400 Bad Request` - Validation failed (missing required fields, invalid values)
- `409 Conflict` - Priority already in use by another rule

**Priority Conflict Example:**
```json
{
  "error": "grouping rule priority already in use",
  "details": {
    "priority": 10,
    "conflicting_id": "550e8400-e29b-41d4-a716-446655440000",
    "conflicting_name": "Existing Rule Name"
  }
}
```

---

### Update Grouping Rule

**PUT** `/api/v1/grouping-rules/:id`

Updates an existing grouping rule. All fields are optional - only provide fields you want to update.

**Path Parameters:**
- `id` (required): UUID of the grouping rule

**Request Body (all fields optional):**
```json
{
  "name": "Updated Name",
  "description": "Updated description",
  "enabled": false,
  "priority": 60,
  "match_labels": {
    "service": "*"
  },
  "cross_source_labels": ["service", "env", "region"],
  "time_window_seconds": 1200
}
```

**Example Request:**
```bash
curl -X PUT http://localhost:8080/api/v1/grouping-rules/550e8400-e29b-41d4-a716-446655440000 \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": false,
    "description": "Disabled for testing"
  }'
```

**Success Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Cross-source: Group by service and env",
  "description": "Disabled for testing",
  "enabled": false,
  "priority": 50,
  "match_labels": {
    "severity": "critical"
  },
  "cross_source_labels": ["service", "env"],
  "time_window_seconds": 600,
  "created_at": "2026-02-12T10:00:00Z",
  "updated_at": "2026-02-12T15:45:00Z"
}
```

**Error Responses:**
- `400 Bad Request` - Validation failed
- `404 Not Found` - Rule with specified ID does not exist
- `409 Conflict` - New priority already in use by another rule

---

### Delete Grouping Rule

**DELETE** `/api/v1/grouping-rules/:id`

Deletes a grouping rule.

**Path Parameters:**
- `id` (required): UUID of the grouping rule

**Example Request:**
```bash
curl -X DELETE http://localhost:8080/api/v1/grouping-rules/550e8400-e29b-41d4-a716-446655440000
```

**Success Response (204 No Content):**
No response body.

**Error Responses:**
- `404 Not Found` - Rule with specified ID does not exist
- `400 Bad Request` - Invalid UUID format

---

## Complete Workflow Example

### 1. Create a Service-Level Grouping Rule

```bash
curl -X POST http://localhost:8080/api/v1/grouping-rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Service-level incidents",
    "description": "Group all alerts for the same service/environment",
    "enabled": true,
    "priority": 50,
    "match_labels": {
      "service": "*"
    },
    "cross_source_labels": ["service", "env"],
    "time_window_seconds": 600
  }'
```

Response:
```json
{
  "id": "abc123...",
  "name": "Service-level incidents",
  ...
}
```

### 2. Send Test Alerts

Send alerts from different sources with same service/env:

```bash
# Prometheus alert
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -d '{
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighCPU",
        "service": "api",
        "env": "production"
      }
    }]
  }'

# Grafana alert (5 seconds later)
curl -X POST http://localhost:8080/api/v1/webhooks/grafana \
  -d '{
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighLatency",
        "service": "api",
        "env": "production"
      }
    }]
  }'
```

**Expected Result:** Both alerts grouped into same incident!

### 3. List Active Rules

```bash
curl http://localhost:8080/api/v1/grouping-rules?enabled=true
```

### 4. Disable a Rule Temporarily

```bash
curl -X PUT http://localhost:8080/api/v1/grouping-rules/abc123... \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'
```

### 5. Re-enable the Rule

```bash
curl -X PUT http://localhost:8080/api/v1/grouping-rules/abc123... \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

### 6. Delete the Rule

```bash
curl -X DELETE http://localhost:8080/api/v1/grouping-rules/abc123...
```

---

## Common Use Cases

### Use Case 1: Create High-Priority Rule for Payment Service

Critical alerts for payment service should be grouped aggressively:

```bash
curl -X POST http://localhost:8080/api/v1/grouping-rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Payment service critical alerts",
    "priority": 10,
    "match_labels": {
      "service": "payment-api",
      "severity": "critical"
    },
    "cross_source_labels": ["service", "env", "region"],
    "time_window_seconds": 1800
  }'
```

### Use Case 2: Create Default Fallback Rule

Catch-all rule for ungrouped alerts:

```bash
curl -X POST http://localhost:8080/api/v1/grouping-rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Default: group by alertname",
    "priority": 999,
    "match_labels": {
      "alertname": "*"
    },
    "cross_source_labels": [],
    "time_window_seconds": 300
  }'
```

### Use Case 3: Temporarily Disable All Custom Rules

Get all custom rules and disable them:

```bash
# List all enabled rules
RULES=$(curl -s http://localhost:8080/api/v1/grouping-rules?enabled=true | jq -r '.data[].id')

# Disable each rule
for RULE_ID in $RULES; do
  curl -X PUT http://localhost:8080/api/v1/grouping-rules/$RULE_ID \
    -H "Content-Type: application/json" \
    -d '{"enabled": false}'
done
```

---

## Best Practices

### 1. Use Priority for Rule Ordering

- **1-10**: Critical/high-priority services (payment, auth)
- **11-50**: Service-specific rules
- **51-100**: Environment-specific rules
- **101-900**: General purpose rules
- **901-999**: Fallback rules

### 2. Test Before Enabling

Create rules with `enabled: false`, test with sample alerts, then enable:

```bash
# 1. Create disabled rule
curl -X POST http://localhost:8080/api/v1/grouping-rules \
  -d '{"name": "Test Rule", "enabled": false, ...}'

# 2. Send test alerts and check grouping logs

# 3. Enable the rule
curl -X PUT http://localhost:8080/api/v1/grouping-rules/$RULE_ID \
  -d '{"enabled": true}'
```

### 3. Use Descriptive Names

✅ Good: `"Critical production API alerts grouped by service"`
❌ Bad: `"Rule 1"`

### 4. Document Complex Rules

Use the `description` field to explain why the rule exists:

```json
{
  "name": "Payment service alerts",
  "description": "Groups payment service alerts aggressively (30min window) because payment incidents often have cascading alerts from different monitoring systems. Created 2026-02-10 after incident INC-245."
}
```

---

## Troubleshooting

### Rule Not Matching Alerts

**Problem:** Created a rule but alerts still create separate incidents.

**Debug Steps:**

1. **Check rule is enabled:**
   ```bash
   curl http://localhost:8080/api/v1/grouping-rules/YOUR_RULE_ID | jq '.enabled'
   ```

2. **Check rule priority:**
   ```bash
   curl http://localhost:8080/api/v1/grouping-rules | jq '.data | sort_by(.priority) | .[].name'
   ```
   Lower priority rules might be matching first.

3. **Check match_labels:**
   Does the alert have the labels specified in `match_labels`?

4. **Check grouping engine cache:**
   Rules are cached for 30 seconds. Wait 30s after creating/updating a rule.

### Priority Conflict

**Problem:** Getting 409 Conflict when creating/updating a rule.

**Solution:** Check which rule is using that priority:

```bash
curl http://localhost:8080/api/v1/grouping-rules | \
  jq '.data[] | select(.priority == 50) | {id, name, priority}'
```

Update your rule to use a different priority, or update the conflicting rule first.

---

## Related Documentation

- **[cross-source-correlation.md](cross-source-correlation.md)** - How cross-source grouping works
- **[EPIC-013-PROGRESS.md](../EPIC-013-PROGRESS.md)** - Implementation details
- **[webhooks.md](webhooks.md)** - Webhook payload formats

---

**Status:** OI-104 ✅ Complete (v0.3)

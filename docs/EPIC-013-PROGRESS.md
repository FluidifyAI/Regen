# Epic 013: Enhanced Alert Deduplication & Grouping Rules — Progress Report

**Started:** February 11, 2026
**Implementation Plan:** [docs/plans/2026-02-09-v0.3-multi-source-alerts.md](plans/2026-02-09-v0.3-multi-source-alerts.md)

---

## Completed Tasks

### ✅ Phase 1: OI-100 - Grouping Rules Schema + Migration

**Files Created:**
- `backend/internal/models/grouping_rule.go` (155 lines)
- `backend/internal/repository/grouping_rule_repository.go` (201 lines)
- `backend/migrations/000007_create_grouping_rules_table.up.sql`
- `backend/migrations/000007_create_grouping_rules_table.down.sql`

**Key Features:**
- **JSONB match_labels** for flexible label matching (`{"service": "*"}` matches any service)
- **Priority-based evaluation** with UNIQUE constraint (lower number = higher priority)
- **Time window support** (default 300 seconds / 5 minutes)
- **Cross-source labels** for correlating alerts from different monitoring systems
- **GIN index** on match_labels for efficient JSONB queries
- **Default rule seeded**: "Group by alertname within 5 minutes"

**Repository Operations:**
- Create, GetByID, GetAll, GetEnabled, Update, Delete
- Priority conflict detection
- Validation (name, priority, time window)

---

### ✅ Phase 2: OI-101 - Grouping Engine Implementation

**Files Created:**
- `backend/internal/services/grouping_engine.go` (280 lines)

**Key Features:**
- **Rule cache with 30s TTL** - Avoids database hits on every alert
- **First-match-wins evaluation** - Rules evaluated in priority order
- **Wildcard matching** - `"*"` matches any label value (but label must exist)
- **Deterministic group key generation** - SHA256(sorted_labels)
- **Advisory lock support** - Prevents race conditions

**Core Methods:**
```go
EvaluateAlert(alert *Alert) (*GroupingDecision, error)
RefreshRules() error
matchesRule(alertLabels map[string]string, rule *GroupingRule) bool
deriveGroupKey(alertLabels map[string]string, rule *GroupingRule) string
findOpenIncidentForGroup(groupKey string, timeWindowSeconds int) (*Incident, error)
```

**GroupingDecision Actions:**
- `GroupActionCreateNew` - Create new incident with group_key
- `GroupActionLinkToExisting` - Link alert to existing incident
- `GroupActionDefault` - No rule matched, use default behavior

---

### ✅ Phase 2: OI-102 - Pipeline Integration

**Files Modified:**
- `backend/internal/models/incident.go` - Added `GroupKey *string` field
- `backend/internal/services/alert_service.go` - Integrated grouping engine into pipeline
- `backend/internal/services/incident_service.go` - Added grouping methods
- `backend/internal/services/grouping_engine.go` - Updated to use group_key field
- `backend/internal/services/slack_message_builder.go` - Added BuildAlertLinkedMessage()

**Files Created:**
- `backend/migrations/000008_add_group_key_to_incidents.up.sql`
- `backend/migrations/000008_add_group_key_to_incidents.down.sql`

**Key Features:**

#### 1. Database Schema Changes
- Added `group_key VARCHAR(64)` to incidents table
- Created composite index: `idx_incidents_group_key_status_created`
- Nullable field (manually created incidents have NULL group_key)

#### 2. AlertService Integration
```go
// New pipeline flow in ProcessNormalizedAlerts():
1. Convert NormalizedAlert → models.Alert
2. Deduplicate (source, external_id)
3. If new alert + should create incident:
   a. Evaluate grouping rules (if engine configured)
   b. Based on decision:
      - Link to existing incident
      - Create with group_key
      - Create without group_key (default)
```

#### 3. IncidentService Enhancements
**New Methods:**
- `CreateIncidentFromAlertWithGrouping(alert, groupKey)` - Creates incident with group_key
- `LinkAlertToExistingIncident(alert, incidentID)` - Links alert to existing incident
- `linkAlertToIncidentInTx(tx, alert, incident)` - Helper for transaction-safe linking

**Race Condition Prevention:**
```go
// PostgreSQL advisory lock prevents concurrent incident creation
1. Acquire advisory lock on group_key hash
2. Double-check if incident was created while waiting for lock
3. If yes → link alert to existing incident
4. If no → create new incident
5. Lock auto-released on transaction commit
```

#### 4. Slack Integration
- **New message type**: `BuildAlertLinkedMessage(alert, incident)`
- Posted when alert is linked to existing incident
- Shows alert details in incident channel

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                   Alert Processing Pipeline                  │
└─────────────────────────────────────────────────────────────┘
                              ↓
                    ┌─────────────────┐
                    │ Webhook Arrives │
                    └────────┬────────┘
                             ↓
                    ┌─────────────────┐
                    │  Parse & Dedupe │
                    └────────┬────────┘
                             ↓
              ┌──────────────┴──────────────┐
              │ Should Create Incident?     │
              └──────────────┬──────────────┘
                             ↓ Yes
              ┌──────────────┴──────────────┐
              │  Grouping Engine Configured? │
              └──────────────┬──────────────┘
                             ↓ Yes
              ┌──────────────┴──────────────┐
              │   EvaluateAlert()           │
              │   (Load cached rules)       │
              └──────────────┬──────────────┘
                             ↓
        ┌────────────────────┼────────────────────┐
        │                    │                    │
   Rule Match?          Rule Match?         No Match
   Group Found          No Group Found      (Default)
        │                    │                    │
        ↓                    ↓                    ↓
┌───────────────┐  ┌────────────────────┐  ┌──────────────┐
│ Link to       │  │ Create with        │  │ Create       │
│ Existing      │  │ group_key          │  │ without      │
│ Incident      │  │ (Advisory Lock)    │  │ group_key    │
└───────────────┘  └────────────────────┘  └──────────────┘
        │                    │                    │
        ↓                    ↓                    ↓
   Slack Thread        Slack Channel       Slack Channel
   Notification        Created             Created
```

---

## Database Migrations

### Migration 000007: Grouping Rules Table
```sql
CREATE TABLE grouping_rules (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL,
    match_labels JSONB NOT NULL DEFAULT '{}',
    time_window_seconds INTEGER NOT NULL DEFAULT 300,
    cross_source_labels JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT grouping_rules_priority_unique UNIQUE (priority)
);

-- Indexes
CREATE INDEX idx_grouping_rules_enabled_priority ON grouping_rules (enabled, priority) WHERE enabled = true;
CREATE INDEX idx_grouping_rules_match_labels ON grouping_rules USING GIN (match_labels);

-- Seed default rule
INSERT INTO grouping_rules (name, description, priority, match_labels, time_window_seconds)
VALUES ('Default: group by alertname', '...', 100, '{"alertname": "*"}', 300);
```

### Migration 000008: Add Group Key to Incidents
```sql
ALTER TABLE incidents ADD COLUMN group_key VARCHAR(64);

CREATE INDEX idx_incidents_group_key_status_created
ON incidents (group_key, status, created_at)
WHERE group_key IS NOT NULL;
```

---

## Testing the Implementation

### Manual Test Scenarios

#### Scenario 1: Default Grouping (by alertname)
```bash
# Send 3 alerts with same alertname within 5 minutes
curl -X POST http://localhost:8080/api/v1/webhooks/generic \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [
      {"title": "HighCPU", "labels": {"instance": "web-01"}},
      {"title": "HighCPU", "labels": {"instance": "web-02"}},
      {"title": "HighCPU", "labels": {"instance": "web-03"}}
    ]
  }'

# Expected Result:
# - 3 alerts created
# - 1 incident created (all 3 alerts grouped)
# - 1 Slack channel
# - 2 additional Slack thread notifications (alert 2 & 3)
```

#### Scenario 2: No Grouping (different alertnames)
```bash
curl -X POST http://localhost:8080/api/v1/webhooks/generic \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [
      {"title": "HighCPU"},
      {"title": "HighMemory"},
      {"title": "DiskFull"}
    ]
  }'

# Expected Result:
# - 3 alerts created
# - 3 incidents created (no grouping - different alertnames)
# - 3 Slack channels
```

#### Scenario 3: Custom Grouping Rule
```sql
-- Add custom rule: Group by service and env
INSERT INTO grouping_rules (name, priority, match_labels, time_window_seconds)
VALUES ('Group by service+env', 50, '{"service": "*", "env": "*"}', 600);

-- Send alerts with same service+env
-- Expected: Grouped even if different alertnames
```

---

---

### ✅ Phase 3: OI-103 - Cross-Source Alert Correlation

**Completed:** February 12, 2026

**Files Modified:**
- `backend/internal/models/alert.go` - Added `JSONBArray` type for JSONB array fields
- `backend/internal/models/grouping_rule.go` - Changed `CrossSourceLabels` type from `JSONB` to `JSONBArray`
- `backend/internal/services/grouping_engine.go` - Updated `deriveGroupKey()` to support cross-source correlation

**Files Created:**
- `backend/internal/services/grouping_cross_source_test.go` (224 lines) - Unit tests for cross-source correlation
- `docs/examples/cross-source-correlation.md` - Comprehensive documentation and examples

**Key Features:**

#### 1. Cross-Source Label Support
The grouping engine now properly uses `cross_source_labels` to derive group keys:
- If `cross_source_labels` is specified, use ONLY those labels for grouping
- If `cross_source_labels` is empty, fall back to `match_labels` (backward compatible)
- Enables alerts from different sources to be grouped together

**Example:**
```go
// Rule: cross_source_labels = ["service", "env"]
// Alert 1 (Prometheus): {alertname: "HighCPU", service: "api", env: "prod"}
// Alert 2 (Grafana): {alertname: "HighLatency", service: "api", env: "prod"}
// Result: Both alerts get the SAME group key → grouped into 1 incident
```

#### 2. JSONBArray Type
Created new `JSONBArray` type for PostgreSQL JSONB array fields:
```go
type JSONBArray []string

func (j JSONBArray) Value() (driver.Value, error) { ... }
func (j *JSONBArray) Scan(value interface{}) error { ... }
```

This allows proper serialization/deserialization of string arrays in JSONB columns.

#### 3. Updated Group Key Derivation Algorithm

**Before:**
- Always used `match_labels` keys to derive group key
- No way to correlate alerts from different sources

**After:**
```go
func deriveGroupKey(alertLabels map[string]string, rule *GroupingRule) string {
    var keysToInclude []string

    // Use cross_source_labels if specified, otherwise fall back to match_labels
    if len(rule.CrossSourceLabels) > 0 {
        keysToInclude = rule.CrossSourceLabels
    } else {
        // Extract keys from match_labels (backward compatible)
        for k := range matchLabels {
            keysToInclude = append(keysToInclude, k)
        }
    }

    // Sort keys, build "key1=value1|key2=value2", hash with SHA256
    ...
}
```

#### 4. Comprehensive Test Coverage

**Test Scenarios:**
- ✅ `TestCrossSourceGroupKeyDerivation` - Verifies alerts from 3 different sources (Prometheus, Grafana, CloudWatch) get the same group key
- ✅ `TestCrossSourceGroupKeyDerivation_DifferentEnv` - Verifies different env values create different group keys
- ✅ `TestFallbackToMatchLabels_WhenNoCrossSourceLabels` - Verifies backward compatibility
- ✅ `TestEmptyCrossSourceLabels_UsesMatchLabels` - Verifies empty array falls back to match_labels
- ✅ `TestCrossSourceMultipleLabels` - Verifies multiple cross_source_labels work correctly

All tests passing ✅

---

### ✅ Phase 4: OI-104 - Grouping Rules CRUD API

**Completed:** February 12, 2026

**Files Created:**
- `backend/internal/api/handlers/dto/grouping_rule_request.go` (86 lines) - Request DTOs
- `backend/internal/api/handlers/dto/grouping_rule_response.go` (39 lines) - Response DTOs
- `backend/internal/api/handlers/grouping_rules.go` (263 lines) - REST API handlers
- `backend/internal/api/handlers/grouping_rules_test.go` (332 lines) - API tests
- `docs/examples/grouping-rules-api.md` (600+ lines) - API documentation

**Files Modified:**
- `backend/internal/api/routes.go` - Added grouping rules routes

**Key Features:**

#### 1. Complete REST API
All CRUD operations implemented:
- **GET** `/api/v1/grouping-rules` - List rules (with optional `enabled` filter)
- **GET** `/api/v1/grouping-rules/:id` - Get single rule
- **POST** `/api/v1/grouping-rules` - Create new rule
- **PUT** `/api/v1/grouping-rules/:id` - Update existing rule
- **DELETE** `/api/v1/grouping-rules/:id` - Delete rule

#### 2. Request/Response DTOs
Clean separation between API and internal models:
```go
type CreateGroupingRuleRequest struct {
    Name              string   `json:"name" binding:"required,min=1,max=255"`
    Priority          int      `json:"priority" binding:"required,min=1,max=1000"`
    MatchLabels       JSONB    `json:"match_labels" binding:"required"`
    CrossSourceLabels []string `json:"cross_source_labels"`
    TimeWindowSeconds int      `json:"time_window_seconds" binding:"required,min=1,max=86400"`
    // ...
}
```

#### 3. Validation
Input validation using Gin binding tags:
- Name: 1-255 characters (required)
- Description: Max 1000 characters
- Priority: 1-1000, must be unique
- TimeWindowSeconds: 1-86400 (1 second to 24 hours)
- MatchLabels: Required, cannot be empty

#### 4. Priority Conflict Detection
Prevents duplicate priorities:
```json
// 409 Conflict response
{
  "error": "grouping rule priority already in use",
  "details": {
    "priority": 50,
    "conflicting_id": "550e8400-...",
    "conflicting_name": "Existing Rule Name"
  }
}
```

#### 5. Comprehensive Documentation
- Complete API reference with examples
- Common use cases and workflows
- Best practices for rule management
- Troubleshooting guide

---

### ✅ Phase 5: OI-105 - Grouped Alerts UI

**Completed:** February 12, 2026

**Files Created:**
- `frontend/src/components/incidents/GroupedAlerts.tsx` (230 lines) - Grouped alerts visualization component
- `docs/examples/grouped-alerts-ui.md` (400+ lines) - UI documentation

**Files Modified:**
- `frontend/src/api/types.ts` - Added `group_key` to Incident interface
- `frontend/src/pages/IncidentDetailPage.tsx` - Replaced AlertsList with GroupedAlerts component

**Key Features:**

#### 1. Grouping Header
Prominent header shown when incident has `group_key`:
- Alert count display
- Source diversity indicator ("3 alerts from 2 different sources")
- Source badges for cross-source correlation

#### 2. Visual Connectors
- Vertical lines connecting grouped alerts
- Brand-colored borders and backgrounds for grouped alerts
- Clear visual distinction between grouped and non-grouped incidents

#### 3. Cross-Source Highlighting
- Source names highlighted in brand color when multiple sources present
- Source badges showing all unique monitoring systems
- Visual emphasis on cross-source correlation

#### 4. Enhanced Alert Cards
Each alert shows:
- Link icon for grouped alerts
- Alert title and severity
- Expandable labels section (show/hide)
- Source, timestamps, resolved status
- Description with line clamping

#### 5. Advanced Section
- Expandable "Group Key" section for debugging
- Shows SHA256 hash used for grouping
- Explains how grouping works
- Useful for troubleshooting grouping rules

#### 6. Responsive Design
- Clean, minimal interface
- Hover states for interactive elements
- Proper truncation for long text
- Mobile-friendly layout

---

## What's Next

### 🚧 OI-106: Integration Tests
- Test grouping engine with various rule configurations
- Test race conditions with concurrent webhooks
- Test advisory locks
- Test cross-source correlation
- End-to-end testing of full pipeline

---

## Success Metrics (Partial)

✅ **Completed:**
- [x] Database schema for grouping rules
- [x] Grouping engine with rule evaluation
- [x] Pipeline integration with alert processing
- [x] Advisory locks prevent race conditions
- [x] Slack notifications for grouped alerts
- [x] Default rule creates logical groups

✅ **Completed:**
- [x] Cross-source correlation (OI-103)
- [x] CRUD API for managing rules (OI-104)
- [x] UI for viewing grouped alerts (OI-105)

❌ **Not Yet Implemented:**
- [ ] Integration tests (OI-106)

---

## Key Learnings

### 1. PostgreSQL Advisory Locks for Grouping
The use of `pg_advisory_xact_lock()` elegantly solves race conditions:
- Transaction-scoped (auto-release on commit/rollback)
- Works across multiple OpenIncident instances (horizontal scaling)
- No deadlock risk with transaction-scoped locks

### 2. Separation of Concerns
- **GroupingEngine**: Pure decision-making (evaluate, match, derive key)
- **IncidentService**: Transaction management and side effects (Slack, DB)
- **AlertService**: Orchestration and pipeline flow

### 3. Backward Compatibility
- Grouping engine is optional (`groupingEngine` can be nil)
- Existing v0.2 behavior preserved when grouping disabled
- `group_key` is nullable (manually created incidents have NULL)

---

## File Inventory

### New Files (17)
| File | Lines | Purpose |
|------|-------|---------|
| `models/grouping_rule.go` | 155 | GroupingRule data model |
| `repository/grouping_rule_repository.go` | 201 | CRUD operations for rules |
| `services/grouping_engine.go` | 280 | Rule evaluation and grouping logic |
| `services/grouping_cross_source_test.go` | 224 | Cross-source correlation tests |
| `api/handlers/dto/grouping_rule_request.go` | 86 | API request DTOs |
| `api/handlers/dto/grouping_rule_response.go` | 39 | API response DTOs |
| `api/handlers/grouping_rules.go` | 263 | REST API handlers |
| `api/handlers/grouping_rules_test.go` | 332 | API tests |
| `frontend/src/components/incidents/GroupedAlerts.tsx` | 230 | Grouped alerts UI component |
| `docs/examples/cross-source-correlation.md` | 600+ | Cross-source documentation |
| `docs/examples/grouping-rules-api.md` | 600+ | API documentation |
| `docs/examples/grouped-alerts-ui.md` | 400+ | UI documentation |
| `docs/examples/test-grouping-api.sh` | 150 | API test script |
| `migrations/000007_*.sql` | 2 files | Grouping rules table |
| `migrations/000008_*.sql` | 2 files | Group key column |

### Modified Files (11)
| File | Changes |
|------|---------|
| `models/alert.go` | Added `JSONBArray` type for JSONB arrays |
| `models/grouping_rule.go` | Changed `CrossSourceLabels` to `JSONBArray` type |
| `models/incident.go` | Added `GroupKey *string` field |
| `services/alert_service.go` | Integrated grouping engine into pipeline |
| `services/incident_service.go` | Added grouping methods (3 new methods) |
| `services/grouping_engine.go` | Updated `deriveGroupKey()` for cross-source support |
| `services/slack_message_builder.go` | Added BuildAlertLinkedMessage() |
| `api/routes.go` | Added grouping rules API routes |
| `frontend/src/api/types.ts` | Added `group_key` to Incident interface |
| `frontend/src/pages/IncidentDetailPage.tsx` | Integrated GroupedAlerts component |

---

**Phase Status:** OI-100 ✅ | OI-101 ✅ | OI-102 ✅ | OI-103 ✅ | OI-104 ✅ | OI-105 ✅ | OI-106 🚧

**Ready for:** OI-106 (Integration Tests) - Final task for Epic 013

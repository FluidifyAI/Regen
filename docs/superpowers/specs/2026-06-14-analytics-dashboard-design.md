# Analytics Dashboard — Incident Analytics + AI Cost Per User

**Date:** 2026-06-14
**Linear issue:** OPE-184
**Feature tier:** Pro only
**Repos affected:** `FluidifyAI/regen-pro` (primary — all new code lives here)

---

## Overview

Two discrete additions to the existing Analytics page in regen-pro:

1. **Incident Analytics tab** — replaces the `PlaceholderTab` stub with a real view: total incident count, MTTD, MTTR, resolution rate, severity breakdown, status distribution, and a daily trend chart. Backed by a new `GET /api/v1/analytics/incidents` endpoint registered entirely in regen-pro.

2. **AI cost per user** — extends the existing AI & Cost tab with a "By user" section showing per-user spend. Requires a DB migration (add `user_id` to `ai_usage_events`), updating the `Track()` call signature, updating all six AI call sites in OSS handlers to pass the authenticated user, and a JOIN query surfacing user email/name.

---

## Design Questions — Resolved

### Q1: Where does the analytics endpoint live?

**Decision: Register in regen-pro via the enterprise hook mechanism.**

The `CostTracker` interface already shows the pattern: a `RegisterRoutes(group *gin.RouterGroup, db *gorm.DB)` method on a Pro struct, called from OSS routes with a Pro-specific group prefix. We add a parallel `AnalyticsProvider` interface in `enterprise/enterprise.go` with the same shape. The OSS stub returns 402 on all routes. regen-pro implements `AnalyticsProvider` with a real handler.

This keeps all analytics SQL inside regen-pro where it belongs, incident data is read from the shared PostgreSQL database (same DB as OSS), and no analytics logic leaks into the AGPLv3 repo.

**Rejected alternative: add endpoint to OSS.** The endpoint is a Pro feature — it cannot go in the public repo regardless of where the data lives.

### Q2: SQL approach for incident analytics

**Decision: raw SQL via `db.Raw(...)` for the aggregation queries.**

The daily count query requires `DATE_TRUNC('day', created_at AT TIME ZONE 'UTC')` grouping, which GORM's query builder cannot express cleanly. The other aggregates (MTTD, MTTR, resolution rate) require `EXTRACT(EPOCH FROM ...)` arithmetic on nullable timestamp columns. Forcing these into GORM results in unreadable, fragile query composition.

Raw SQL is used in the analytics repository only. GORM struct scanning (`db.Raw(...).Scan(&rows)`) is still used to map result rows to typed structs — this keeps the marshalling safe without sacrificing the ability to write readable SQL.

**Rejected alternative: GORM query builder.** `DATE_TRUNC` and `EXTRACT` require raw expressions; mixing them with GORM's fluent API produces code that is harder to test and read than clean SQL.

### Q3: Chart rendering without a library

**Decision: SVG `<rect>` bars rendered inline in the component.**

The existing UI uses only inline style objects (no Tailwind, no external libraries). SVG gives precise control over bar heights, labels, and tooltip positioning without a dependency. The bars are sized proportionally: `height = (count / maxCount) * MAX_BAR_HEIGHT`. Bar widths and gaps are computed from the SVG `viewBox` width divided by the number of data points.

A tooltip (a `<title>` element inside each `<rect>`, or a small positioned `<div>` on hover managed with a single `useState`) surfaces the exact date and count.

**Rejected alternative: CSS flexbox div-bars.** Div bars cannot easily render x-axis date labels beneath each bar without precise pixel alignment. SVG handles both bars and labels in a single coordinate system.

### Q4: User ID extraction pattern

**Decision: use `middleware.GetLocalUser(c)` from the OSS middleware package, which is already exported.**

The auth middleware (`/mnt/c/Users/inder/OneDrive/Documents/Regen/backend/internal/api/middleware/auth.go`) stores the authenticated user under the context key `"local_user"` as a `*models.User`. `GetLocalUser(c *gin.Context) *models.User` is already exported and used by `RequireAdmin()`.

Each of the six AI handler call sites adds:

```go
var userID *uuid.UUID
if u := middleware.GetLocalUser(c); u != nil {
    uid := u.ID
    userID = &uid
}
```

This value is passed into `enterprise.UsageEvent`. The field is nullable — if auth is in open mode (no user in context), `userID` is `nil` and the event is recorded without user attribution.

**Rejected alternative: read the `"user_id"` string key.** The audit middleware sets a `"user_id"` string key, but the auth middleware stores the full `*models.User` struct under `"local_user"`. `GetLocalUser` is already the canonical way to access the typed user object; using it avoids a string-to-UUID conversion and a separate context key lookup.

### Q5: Migration numbering

**Decision: regen-pro migrations continue from where OSS stopped, using the next available number.**

OSS is currently at `000041`. regen-pro has one Pro migration: `000042_custom_field_definitions`. The new migration is `000043_ai_usage_events_user_id`.

The `mergeFS` in `regen-pro/backend/cmd/regen-pro/main.go` overlays pro migrations on top of OSS migrations in a merged `fs.FS`. Both pools share the same sequential number space so that `golang-migrate` sees them as one ordered list. Using `000043` is correct and keeps the sequence unbroken.

### Q6: `by_user` JOIN strategy

**Decision: single SQL JOIN query.**

```sql
SELECT
    u.id          AS user_id,
    u.email       AS email,
    u.name        AS display_name,
    COALESCE(SUM(e.cost_usd), 0) AS total_usd
FROM ai_usage_events e
JOIN users u ON u.id = e.user_id
WHERE e.cost_usd IS NOT NULL
GROUP BY u.id, u.email, u.name
ORDER BY total_usd DESC;
```

The `users` table is in the same PostgreSQL database as `ai_usage_events`. A JOIN is the correct tool: one round-trip, no N+1, straightforward to test against a test DB.

**Rejected alternative: two-query approach.** Fetching user IDs and amounts first, then doing a second query to look up user details, introduces an N+1 pattern and requires merging results in Go code. No benefit over a single JOIN for this read-only aggregation.

---

## Section 1: What We Are Building

### Incident Analytics tab

Replace the `PlaceholderTab` for `tab === 'incidents'` in `AnalyticsPage.tsx` with a real `IncidentAnalyticsTab` component.

**Backend: `GET /api/v1/analytics/incidents?range=7d|30d|90d`**

Query parameter `range` defaults to `30d`. Supported values: `7d`, `30d`, `90d`.

Response shape:

```json
{
  "total": 42,
  "mttd_minutes": 4.5,
  "mttr_minutes": 38.2,
  "resolution_rate": 0.93,
  "by_severity": {
    "critical": 3,
    "high": 12,
    "medium": 18,
    "low": 9
  },
  "by_status": {
    "triggered": 2,
    "acknowledged": 1,
    "resolved": 39
  },
  "daily": [
    { "date": "2026-05-15", "count": 2 },
    { "date": "2026-05-16", "count": 0 }
  ]
}
```

**MTTD** = average of `(acknowledged_at - created_at)` in minutes, for incidents where `acknowledged_at IS NOT NULL`.
**MTTR** = average of `(resolved_at - created_at)` in minutes, for incidents where `resolved_at IS NOT NULL`.
**Resolution rate** = `count(resolved_at IS NOT NULL) / total`.

The `daily` array always covers the full range window (even zero-count days), sorted ascending by date.

**Frontend: `IncidentAnalyticsTab` component**

- Summary row: four `MetricCard` instances — Total, MTTD, MTTR, Resolution Rate.
- Severity bar section: horizontal bars, one per severity, scaled to the max count. Color-coded using existing `C.critical`, `C.high`, `C.medium`, `C.low` tokens.
- Status distribution: three inline badges with counts.
- Daily trend: inline SVG bar chart, one `<rect>` per day, height proportional to count, date labels beneath every 5th bar (to avoid crowding).
- Time range toggle: three buttons (`7d / 30d / 90d`) in the tab header area, same styling as existing button patterns in the file.

**What this is not:** no SLO tracking, no alert fatigue metrics, no per-service breakdown. Those belong to the Reliability tab (future work).

### AI cost per user

**DB change:** add a nullable `user_id uuid` column to `ai_usage_events`.

**`enterprise.UsageEvent` change:** add `UserID *uuid.UUID` field. The OSS struct definition is the contract; the Pro implementation stores it.

**`RecordUsage` change:** the `UsageEvent` struct gains the field — no signature change to the interface method, only the struct. Call sites pass `UserID` by setting it on the struct literal.

**Six call sites in OSS handlers** (all in `FluidifyAI/Regen`):
- `internal/api/handlers/ai.go`: `SummarizeIncident`, `GenerateHandoffDigest`, `EnhanceIncidentDraft`
- `internal/api/handlers/post_mortems.go`: `GeneratePostMortem`, `EnhancePostMortem`
- The sixth operation `answer_question` is listed in the enterprise comment but not yet implemented as a handler — no call site to update now.

Each call site extracts the user with `middleware.GetLocalUser(c)` and populates `UsageEvent.UserID`.

**`CostSummary` response change:** gains `ByUser []UserCostRow`. `UserCostRow` is defined in the enterprise package:

```go
type UserCostRow struct {
    UserID      uuid.UUID `json:"user_id"`
    Email       string    `json:"email"`
    DisplayName string    `json:"display_name"`
    TotalUSD    float64   `json:"total_usd"`
}
```

The OSS `noopCostTracker.GetSummary` returns an empty `ByUser` slice — no change in behaviour for OSS users.

**Frontend:** new "By user" section added below "By operation" in `AICostTab`. Renders a list of rows identical in style to "By operation" rows — user's display name on the left, dollar amount on the right. Uses `u.display_name` if non-empty, falls back to `u.email`. Only rendered when `by_user` is non-null and non-empty.

**Success criteria (testable):**

1. `GET /api/v1/analytics/incidents?range=30d` returns a valid JSON body with all six keys when incidents exist, and returns `total: 0` with empty `daily` array and null/0 metrics when the incidents table is empty.
2. The `daily` array always contains exactly `range_days` entries (e.g. 30 for `30d`), with `count: 0` for days with no incidents.
3. After a user calls `POST /api/v1/incidents/:id/summarize`, the returned `cost_usd` is non-zero (when AI is configured), and `GET /api/v1/ai/cost/summary` shows a `by_user` entry for that user.
4. Incidents tab renders without console errors; MetricCard values are non-negative; daily SVG chart renders at least 1 bar when `total > 0`.
5. `by_user` is absent from the summary response in the OSS build (no-op returns empty slice, which is marshalled as `null` or `[]`).

---

## Section 2: Technical Approach

### 2.1 New enterprise interface in OSS

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen/backend/enterprise/enterprise.go`

Add to the `Hooks` struct:

```go
Analytics AnalyticsProvider
```

New interface:

```go
type AnalyticsProvider interface {
    RegisterRoutes(group *gin.RouterGroup, db *gorm.DB)
}

type noopAnalytics struct{}
func (noopAnalytics) RegisterRoutes(group *gin.RouterGroup, _ *gorm.DB) {
    group.Any("/*path", func(c *gin.Context) {
        c.JSON(http.StatusPaymentRequired, gin.H{
            "error": "incident analytics require a Fluidify Regen Pro licence",
        })
    })
}
```

Wire into `NewNoOp()`:

```go
Analytics: noopAnalytics{},
```

Add `UserID *uuid.UUID` to `UsageEvent` and `ByUser []UserCostRow` to `CostSummary`. Add `UserCostRow` struct (as defined above).

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen/backend/internal/api/routes.go`

Register the analytics group after the cost group:

```go
analyticsGroup := protected.Group("/analytics")
hooks.Analytics.RegisterRoutes(analyticsGroup, db)
```

### 2.2 OSS AI handler call sites (5 handlers)

Each of the five handler functions gains this block immediately before `hooks.CostTracker.RecordUsage(...)`:

```go
var userID *uuid.UUID
if u := middleware.GetLocalUser(c); u != nil {
    uid := u.ID
    userID = &uid
}
```

The `UsageEvent` literal gains `UserID: userID`.

### 2.3 Pro migration

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/backend/migrations/000043_ai_usage_events_user_id.up.sql`

```sql
ALTER TABLE ai_usage_events
    ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_ai_usage_events_user_id ON ai_usage_events (user_id)
    WHERE user_id IS NOT NULL;
```

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/backend/migrations/000043_ai_usage_events_user_id.down.sql`

```sql
DROP INDEX IF EXISTS idx_ai_usage_events_user_id;
ALTER TABLE ai_usage_events DROP COLUMN IF EXISTS user_id;
```

The column is nullable. Existing rows keep `user_id = NULL`. No backfill.

### 2.4 Pro model and repository changes

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/backend/internal/costtracker/model.go`

Add to `AIUsageEvent`:

```go
UserID *uuid.UUID `gorm:"type:uuid;index"`
```

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/backend/internal/costtracker/repository.go`

- `Store(event AIUsageEvent)` — no signature change; `UserID` is now populated on the struct.
- `GetSummary()` return signature expands to include `byUser []enterprise.UserCostRow`:

```go
GetSummary() (totalUSD float64, currentMonthUSD float64, byOperation map[string]float64, byUser []enterprise.UserCostRow, err error)
```

The `by_user` query:

```sql
SELECT
    u.id          AS user_id,
    u.email       AS email,
    u.name        AS display_name,
    COALESCE(SUM(e.cost_usd), 0) AS total_usd
FROM ai_usage_events e
JOIN users u ON u.id = e.user_id
WHERE e.cost_usd IS NOT NULL
GROUP BY u.id, u.email, u.name
ORDER BY total_usd DESC;
```

Run with `db.Raw(query).Scan(&rows)` where rows is `[]enterprise.UserCostRow`.

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/backend/internal/costtracker/tracker.go`

`GetSummary` assembles the full `enterprise.CostSummary` including `ByUser`.

`RecordUsage` maps `event.UserID` onto `dbEvent.UserID`.

### 2.5 New analytics package in regen-pro

**New directory:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/backend/internal/analytics/`

**File: `model.go`** — response struct:

```go
package analytics

type IncidentAnalyticsResponse struct {
    Total          int                `json:"total"`
    MTTDMinutes    *float64           `json:"mttd_minutes"`
    MTTRMinutes    *float64           `json:"mttr_minutes"`
    ResolutionRate float64            `json:"resolution_rate"`
    BySeverity     map[string]int     `json:"by_severity"`
    ByStatus       map[string]int     `json:"by_status"`
    Daily          []DailyCount       `json:"daily"`
}

type DailyCount struct {
    Date  string `json:"date"` // "YYYY-MM-DD"
    Count int    `json:"count"`
}
```

`MTTDMinutes` and `MTTRMinutes` are pointers — they are `null` in JSON when no acknowledged/resolved incidents exist in the window (prevents misleading `0` values).

**File: `repository.go`** — raw SQL queries against the `incidents` table:

Query 1 — aggregate stats:

```sql
SELECT
    COUNT(*)                                                         AS total,
    COUNT(resolved_at)                                               AS resolved_count,
    AVG(EXTRACT(EPOCH FROM (acknowledged_at - created_at)) / 60.0)
        FILTER (WHERE acknowledged_at IS NOT NULL)                   AS mttd_minutes,
    AVG(EXTRACT(EPOCH FROM (resolved_at - created_at)) / 60.0)
        FILTER (WHERE resolved_at IS NOT NULL)                       AS mttr_minutes
FROM incidents
WHERE created_at >= $1;
```

Query 2 — by_severity breakdown:

```sql
SELECT severity, COUNT(*) AS count
FROM incidents
WHERE created_at >= $1
GROUP BY severity;
```

Query 3 — by_status breakdown:

```sql
SELECT status, COUNT(*) AS count
FROM incidents
WHERE created_at >= $1
GROUP BY status;
```

Query 4 — daily counts:

```sql
SELECT
    TO_CHAR(DATE_TRUNC('day', created_at AT TIME ZONE 'UTC'), 'YYYY-MM-DD') AS date,
    COUNT(*) AS count
FROM incidents
WHERE created_at >= $1
GROUP BY DATE_TRUNC('day', created_at AT TIME ZONE 'UTC')
ORDER BY 1;
```

After fetching the raw daily rows, Go code fills in zero-count days: the handler iterates from `windowStart` to `now` day-by-day, looking up each date in a map built from the DB rows and defaulting to `0` if absent.

**File: `handler.go`** — `GET /analytics/incidents`:

```go
func HandleGetIncidentAnalytics(db *gorm.DB) gin.HandlerFunc { ... }
```

Parses `range` query param (default `30d`), derives `windowStart`, calls repository functions, assembles `IncidentAnalyticsResponse`, returns JSON.

**File: `routes.go`** — satisfies `enterprise.AnalyticsProvider`:

```go
type Provider struct{}

func NewProvider() *Provider { return &Provider{} }

func (p *Provider) RegisterRoutes(group *gin.RouterGroup, db *gorm.DB) {
    group.GET("/incidents", HandleGetIncidentAnalytics(db))
}
```

### 2.6 Wire analytics into regen-pro main

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/backend/cmd/regen-pro/main.go`

Add:

```go
hooks.Analytics = analytics.NewProvider()
```

### 2.7 Frontend changes

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/frontend/src/api/incidentAnalytics.ts` (new)

```typescript
export interface IncidentAnalytics {
  total: number
  mttd_minutes: number | null
  mttr_minutes: number | null
  resolution_rate: number
  by_severity: Record<string, number>
  by_status: Record<string, number>
  daily: { date: string; count: number }[]
}

export async function getIncidentAnalytics(range: '7d' | '30d' | '90d'): Promise<IncidentAnalytics> {
  return apiClient.get<IncidentAnalytics>('/api/v1/analytics/incidents', { range })
}
```

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/frontend/src/api/costTracking.ts` (modified)

Add `UserCostRow` and extend `CostSummary`:

```typescript
export interface UserCostRow {
  user_id: string
  email: string
  display_name: string
  total_usd: number
}

export interface CostSummary {
  total_usd: number
  current_month_usd: number
  by_operation: Record<string, number> | null
  by_user: UserCostRow[] | null   // new field
}
```

**File:** `/mnt/c/Users/inder/OneDrive/Documents/Regen-Pro/frontend/src/pages/AnalyticsPage.tsx` (modified)

- Replace `{tab === 'incidents' && <PlaceholderTab .../>}` with `{tab === 'incidents' && <IncidentAnalyticsTab />}`.
- Add `IncidentAnalyticsTab` component (inline in the same file, consistent with existing pattern).
- `IncidentAnalyticsTab` has local state: `data: IncidentAnalytics | null`, `loading: boolean`, `range: '7d'|'30d'|'90d'` (default `'30d'`). `useEffect` re-fetches on `range` change.
- The SVG daily chart is implemented as a sub-component `DailyChart({ daily })` in the same file.
- Add "By user" section to `AICostTab`: below the "By operation" section, render `summary.by_user` rows when non-null/non-empty. Each row: `<User size={12} />` icon, display name (or email), dollar amount right-aligned. Re-uses the same row div style as "By operation".

---

## Section 3: Test Strategy

### Unit tests (regen-pro backend)

**`analytics/repository_test.go`**

- Seed 5 incidents with known `created_at`, `acknowledged_at`, `resolved_at` values inside the window; 2 outside the window.
- Assert `total = 5`, `mttd_minutes` and `mttr_minutes` are within expected range, `resolution_rate` correct.
- Assert `daily` slice has exactly `range_days` entries with zero-filled gaps.
- Assert `by_severity` map has the correct per-severity counts.
- Test `range=7d`, `range=30d`, `range=90d` to verify window filtering.
- Edge case: empty incidents table → `total=0`, `mttd_minutes=null`, `mttr_minutes=null`, `resolution_rate=0`.

**`costtracker/repository_test.go`** (extend existing)

- After `Store` with a `UserID`, verify `GetSummary` returns that user in `by_user` with the correct `total_usd`.
- Store events from two distinct users; verify both appear, ordered by descending `total_usd`.
- Store an event with `UserID = nil`; verify it does not appear in `by_user`.
- Verify existing tests still pass (non-breaking: `total_usd`, `current_month_usd`, `by_operation` unchanged).

**`costtracker/tracker_test.go`** (extend existing)

- `RecordUsage` called with a non-nil `UserID` → verify the stored row's `user_id` is set.
- `GetSummary` returns the correct `ByUser` slice.

### Integration tests (regen-pro backend)

**`analytics/handler_test.go`**

- `GET /api/v1/analytics/incidents` with no incidents → 200, `total: 0`.
- `GET /api/v1/analytics/incidents?range=7d` with incidents spread across 10 days → only those in last 7 days counted.
- Invalid `range` param → 400 bad request.
- Missing `range` param → defaults to `30d`, 200 response.

### Edge cases to test explicitly

- `acknowledged_at` is NULL for all incidents: `mttd_minutes` field in response is JSON `null`, not `0`.
- `resolved_at` is NULL for all incidents: `mttr_minutes` is JSON `null`, `resolution_rate` is `0.0`.
- `daily` array: the last entry date equals today's date in UTC; the first entry equals `now - range_days + 1`.
- `by_user` with `user_id = NULL` events (pre-migration rows): they are excluded from the JOIN result, not shown as an "anonymous" bucket.
- `by_user` when no AI events have `cost_usd` set (all models unpriced): returns empty slice, not an error.

---

## Section 4: Security Considerations

### Input validation

The `range` query parameter is validated against the allowlist `["7d", "30d", "90d"]` before use. Any other value returns 400. The parameter is never interpolated into SQL — it is used only to derive a `time.Time` value passed as a positional parameter `$1`.

All raw SQL queries use positional parameters (`$1`, etc.) exclusively. No string formatting of user input into query strings.

### Auth requirements

Both new endpoints (`GET /api/v1/analytics/incidents`, `GET /api/v1/ai/cost/summary` with the extended `by_user` field) are registered under the `protected` group in OSS routes.go, which applies `middleware.RequireAuth`. No new auth surface is introduced.

`user_id` in `UsageEvent` is populated from the server-side `middleware.GetLocalUser(c)` result — it is never taken from the request body or query parameters.

### Data sensitivity

- The `by_user` aggregation exposes user email addresses and display names to anyone who can call `GET /api/v1/ai/cost/summary`. This endpoint already requires authentication. Since all authenticated users in the current OSS model are effectively admins (single implicit role), this is acceptable. When RBAC is implemented (Pro enterprise tier), this endpoint should be restricted to admin role.
- The `incidents` analytics response is aggregate data only — no PII, no incident content. Low sensitivity.

### Foreign key integrity

The `user_id` column references `users(id) ON DELETE SET NULL`. If a user is deleted from the system, their historical AI usage events are retained with `user_id = NULL` (excluded from `by_user` aggregation). No cost data is lost; attribution is simply removed.

---

## File Manifest

### Modified in OSS (`FluidifyAI/Regen`)

| File | Change |
|------|--------|
| `backend/enterprise/enterprise.go` | Add `AnalyticsProvider` interface + no-op, add `UserID *uuid.UUID` to `UsageEvent`, add `ByUser []UserCostRow` to `CostSummary`, add `UserCostRow` struct, add `Analytics AnalyticsProvider` to `Hooks`, wire no-op in `NewNoOp()` |
| `backend/internal/api/routes.go` | Register `analyticsGroup` under `protected`, call `hooks.Analytics.RegisterRoutes(analyticsGroup, db)` |
| `backend/internal/api/handlers/ai.go` | 3 handlers: extract user from context, populate `UsageEvent.UserID` |
| `backend/internal/api/handlers/post_mortems.go` | 2 handlers: same pattern |

### New/Modified in regen-pro (`FluidifyAI/regen-pro`)

| File | Change |
|------|--------|
| `backend/migrations/000043_ai_usage_events_user_id.up.sql` | New |
| `backend/migrations/000043_ai_usage_events_user_id.down.sql` | New |
| `backend/internal/costtracker/model.go` | Add `UserID *uuid.UUID` to `AIUsageEvent` |
| `backend/internal/costtracker/repository.go` | Extend `Repository` interface and impl: `GetSummary` returns `byUser`; `Store` preserves `UserID` |
| `backend/internal/costtracker/tracker.go` | Propagate `UserID` in `RecordUsage`; assemble `ByUser` in `GetSummary` |
| `backend/internal/costtracker/handlers_test.go` | Extend existing tests |
| `backend/internal/costtracker/repository_test.go` | Extend existing tests |
| `backend/internal/costtracker/tracker_test.go` | Extend existing tests |
| `backend/internal/analytics/model.go` | New: `IncidentAnalyticsResponse`, `DailyCount` |
| `backend/internal/analytics/repository.go` | New: raw SQL queries against `incidents` table |
| `backend/internal/analytics/repository_test.go` | New: unit tests with seeded data |
| `backend/internal/analytics/handler.go` | New: `HandleGetIncidentAnalytics` |
| `backend/internal/analytics/handler_test.go` | New: integration tests |
| `backend/internal/analytics/routes.go` | New: `Provider` struct implementing `enterprise.AnalyticsProvider` |
| `backend/cmd/regen-pro/main.go` | Wire `hooks.Analytics = analytics.NewProvider()` |
| `frontend/src/api/incidentAnalytics.ts` | New: `getIncidentAnalytics`, `IncidentAnalytics` type |
| `frontend/src/api/costTracking.ts` | Add `UserCostRow`, extend `CostSummary` with `by_user` |
| `frontend/src/pages/AnalyticsPage.tsx` | Add `IncidentAnalyticsTab`, `DailyChart`, "By user" section in `AICostTab` |

---

## Out of Scope

- On-Call analytics tab (still `PlaceholderTab`)
- Reliability tab (still `PlaceholderTab`)
- Per-user cost filtering or date-range filtering in the cost summary
- Budget caps or spend alerts
- Backfilling `user_id` on historical `ai_usage_events` rows (pre-migration rows remain with `NULL`)
- The `answer_question` AI operation (not yet implemented as a handler)
- RBAC gate on the analytics or cost endpoints (deferred to the RBAC Pro feature)

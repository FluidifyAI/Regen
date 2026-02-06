# Epic 005: Incident REST API - Implementation Plan

**Date:** 2026-02-06
**Epic:** OI-EPIC-005
**Status:** In Progress
**Phase:** v0.1 - Foundation

---

## 1. Overview

### Objective
Implement REST API endpoints for incident CRUD operations and status transitions, completing the v0.1 MVP feature set for incident management.

### Definition of Done
- All v0.1 incident endpoints functional: list, get, create (manual), update (acknowledge/resolve)
- Proper request validation with field-level error messages
- Standardized error responses across all endpoints
- Status transition validation (triggered→acknowledged→resolved)
- Timeline entries for all state changes
- Slack notifications for status updates
- Request ID tracking for debugging

### Context
**What exists:**
- ✅ Incident model, repository, service layer
- ✅ Timeline model, repository
- ✅ Slack integration (Epic 004)
- ✅ Prometheus webhook handler pattern (Epic 003)
- ✅ Health check endpoints
- ✅ Middleware (CORS, Recovery, Logger)

**What's missing:**
- ❌ HTTP handlers for incident CRUD
- ❌ Request/response DTOs
- ❌ Validation utilities
- ❌ Error handling utilities
- ❌ Status update Slack notifications
- ❌ Request ID middleware

### Success Criteria
```bash
# End-to-end flow works:
1. POST /api/v1/incidents → Creates manual incident → Slack channel created
2. GET /api/v1/incidents → Lists all incidents with pagination
3. GET /api/v1/incidents/{id} → Returns incident with linked alerts and timeline
4. PATCH /api/v1/incidents/{id} → Updates status to acknowledged → Slack notified
5. PATCH /api/v1/incidents/{id} → Updates status to resolved → Slack notified
6. GET /api/v1/incidents/{id}/timeline → Returns chronological timeline
7. POST /api/v1/incidents/{id}/timeline → Adds user note → Shows in timeline

# Error handling works:
- Invalid JSON → 400 with clear error
- Missing required field → 400 with field-level details
- Invalid status transition → 400 with explanation
- Incident not found → 404 with resource type
- Internal error → 500 with safe message, logs full error
- All responses include request_id for tracing
```

---

## 2. Architecture Decisions

### ADR-008: Request/Response DTO Pattern

**Status:** Approved
**Date:** 2026-02-06

**Context:**
We need to expose incidents via REST API with validation, pagination, and filtering. Direct exposure of database models leads to:
- Coupling API to database schema
- Exposing internal fields (CreatedAt, UpdatedAt managed by GORM)
- No validation at API boundary
- Breaking changes when models evolve

**Decision:**
Use separate DTOs (Data Transfer Objects) for API requests and responses:

```
backend/internal/api/handlers/
├── incidents.go              # HTTP handlers
└── dto/
    ├── incident_request.go   # CreateIncidentRequest, UpdateIncidentRequest
    ├── incident_response.go  # IncidentResponse, IncidentListResponse
    ├── timeline_request.go   # CreateTimelineEntryRequest
    ├── timeline_response.go  # TimelineEntryResponse
    ├── pagination.go         # PaginationParams, PaginatedResponse
    └── errors.go             # ErrorResponse, ValidationError
```

**Benefits:**
- API stability independent of database changes
- Clear validation rules at API boundary
- Documentation-friendly (matches OpenAPI schemas)
- Type-safe request parsing

**Example:**
```go
// Request DTO with validation
type CreateIncidentRequest struct {
    Title       string             `json:"title" binding:"required,min=1,max=500"`
    Severity    string             `json:"severity" binding:"omitempty,oneof=critical high medium low"`
    Description string             `json:"description"`
}

// Response DTO with computed fields
type IncidentResponse struct {
    ID             uuid.UUID  `json:"id"`
    IncidentNumber int        `json:"incident_number"`
    Title          string     `json:"title"`
    Status         string     `json:"status"`
    Severity       string     `json:"severity"`
    SlackChannel   *SlackChannelInfo `json:"slack_channel,omitempty"`
    CreatedAt      time.Time  `json:"created_at"`
    // ... computed fields like duration, responder count, etc.
}
```

### ADR-009: Centralized Error Handling

**Status:** Approved
**Date:** 2026-02-06

**Context:**
Consistent error responses are critical for:
- Client error handling
- Debugging (request tracing)
- Security (no leaked internals in 500 errors)
- Developer experience

**Decision:**
Standardize on this error response format:

```go
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code      string                 `json:"code"`       // "validation_error", "not_found"
    Message   string                 `json:"message"`    // Human-readable
    Details   map[string]interface{} `json:"details,omitempty"` // Field errors
    RequestID string                 `json:"request_id"` // For tracing
}
```

**Implementation:**
1. Add request ID middleware (generates UUID per request)
2. Create error response builders in `dto/errors.go`
3. Use consistent status codes:
   - 400 Bad Request → Validation errors
   - 404 Not Found → Resource not found
   - 409 Conflict → Invalid state transition
   - 500 Internal Server Error → Unexpected errors (logs full error, returns safe message)

**Example responses:**
```json
// 400 - Validation error
{
  "error": {
    "code": "validation_error",
    "message": "Invalid request parameters",
    "details": {
      "title": "field is required",
      "severity": "must be one of: critical, high, medium, low"
    },
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
  }
}

// 404 - Not found
{
  "error": {
    "code": "not_found",
    "message": "Incident not found",
    "details": {
      "resource": "incident",
      "id": "123e4567-e89b-12d3-a456-426614174000"
    },
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
  }
}

// 409 - Invalid state transition
{
  "error": {
    "code": "invalid_state_transition",
    "message": "Cannot transition from resolved to acknowledged",
    "details": {
      "current_status": "resolved",
      "requested_status": "acknowledged",
      "valid_transitions": ["resolved"]
    },
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

### ADR-010: Incident ID Resolution (UUID vs Incident Number)

**Status:** Approved
**Date:** 2026-02-06

**Context:**
Users want to reference incidents as:
- `INC-042` (human-friendly incident number)
- UUID (machine-friendly unique ID)

API endpoints need to support both formats.

**Decision:**
`GET /api/v1/incidents/:id` accepts both:
- UUID format → Lookup by UUID
- Numeric format → Lookup by incident_number
- Detection: `strconv.Atoi(id)` succeeds → incident_number, else UUID

**Implementation:**
```go
func parseIncidentIdentifier(id string) (uuid.UUID, int, error) {
    // Try parsing as incident number first (simpler)
    if num, err := strconv.Atoi(id); err == nil {
        return uuid.Nil, num, nil
    }

    // Try parsing as UUID
    if uid, err := uuid.Parse(id); err == nil {
        return uid, 0, nil
    }

    return uuid.Nil, 0, fmt.Errorf("invalid incident identifier")
}
```

### ADR-011: Status Transition Validation

**Status:** Approved
**Date:** 2026-02-06

**Context:**
Incident status follows a workflow. Invalid transitions should be rejected:
- ✅ triggered → acknowledged
- ✅ triggered → resolved
- ✅ acknowledged → resolved
- ✅ triggered → canceled
- ❌ resolved → acknowledged (can't un-resolve)
- ❌ canceled → acknowledged

**Decision:**
Implement explicit state machine validation in handler layer:

```go
var validTransitions = map[models.IncidentStatus][]models.IncidentStatus{
    models.IncidentStatusTriggered: {
        models.IncidentStatusAcknowledged,
        models.IncidentStatusResolved,
        models.IncidentStatusCanceled,
    },
    models.IncidentStatusAcknowledged: {
        models.IncidentStatusResolved,
    },
    models.IncidentStatusResolved: {
        // Terminal state - no transitions
    },
    models.IncidentStatusCanceled: {
        // Terminal state - no transitions
    },
}

func isValidTransition(current, requested models.IncidentStatus) bool {
    if current == requested {
        return true // No-op is valid
    }

    allowed := validTransitions[current]
    for _, status := range allowed {
        if status == requested {
            return true
        }
    }
    return false
}
```

---

## 3. Task Breakdown

### Task 1: Create DTOs and Validation Utilities (OI-035 partial)

**Files:**
- `backend/internal/api/handlers/dto/errors.go` (NEW)
- `backend/internal/api/handlers/dto/pagination.go` (NEW)
- `backend/internal/api/handlers/dto/incident_request.go` (NEW)
- `backend/internal/api/handlers/dto/incident_response.go` (NEW)
- `backend/internal/api/handlers/dto/timeline_request.go` (NEW)
- `backend/internal/api/handlers/dto/timeline_response.go` (NEW)

**Implementation:**

**dto/errors.go:**
```go
package dto

import (
    "github.com/gin-gonic/gin"
    "github.com/go-playground/validator/v10"
)

type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code      string                 `json:"code"`
    Message   string                 `json:"message"`
    Details   map[string]interface{} `json:"details,omitempty"`
    RequestID string                 `json:"request_id"`
}

// Error response builders
func BadRequest(c *gin.Context, message string, details map[string]interface{}) {
    c.JSON(400, ErrorResponse{
        Error: ErrorDetail{
            Code:      "bad_request",
            Message:   message,
            Details:   details,
            RequestID: c.GetString("request_id"),
        },
    })
}

func ValidationError(c *gin.Context, err error) {
    details := make(map[string]interface{})

    // Extract field-level errors from validator
    if ve, ok := err.(validator.ValidationErrors); ok {
        for _, fe := range ve {
            details[fe.Field()] = formatValidationError(fe)
        }
    } else {
        details["error"] = err.Error()
    }

    c.JSON(400, ErrorResponse{
        Error: ErrorDetail{
            Code:      "validation_error",
            Message:   "Invalid request parameters",
            Details:   details,
            RequestID: c.GetString("request_id"),
        },
    })
}

func NotFound(c *gin.Context, resource string, id interface{}) {
    c.JSON(404, ErrorResponse{
        Error: ErrorDetail{
            Code:    "not_found",
            Message: resource + " not found",
            Details: map[string]interface{}{
                "resource": resource,
                "id":       id,
            },
            RequestID: c.GetString("request_id"),
        },
    })
}

func Conflict(c *gin.Context, message string, details map[string]interface{}) {
    c.JSON(409, ErrorResponse{
        Error: ErrorDetail{
            Code:      "conflict",
            Message:   message,
            Details:   details,
            RequestID: c.GetString("request_id"),
        },
    })
}

func InternalError(c *gin.Context, err error) {
    // Log full error for debugging
    slog.Error("internal server error",
        "error", err,
        "path", c.Request.URL.Path,
        "request_id", c.GetString("request_id"),
    )

    // Return safe message to client
    c.JSON(500, ErrorResponse{
        Error: ErrorDetail{
            Code:      "internal_error",
            Message:   "Internal server error",
            RequestID: c.GetString("request_id"),
        },
    })
}

func formatValidationError(fe validator.FieldError) string {
    switch fe.Tag() {
    case "required":
        return "field is required"
    case "min":
        return fmt.Sprintf("must be at least %s", fe.Param())
    case "max":
        return fmt.Sprintf("must be at most %s", fe.Param())
    case "oneof":
        return fmt.Sprintf("must be one of: %s", fe.Param())
    default:
        return fe.Error()
    }
}
```

**dto/pagination.go:**
```go
package dto

type PaginationParams struct {
    Page     int `form:"page" binding:"omitempty,min=1"`
    PageSize int `form:"limit" binding:"omitempty,min=1,max=250"`
    Offset   int `form:"offset" binding:"omitempty,min=0"`
}

func (p *PaginationParams) Normalize() {
    if p.Page == 0 {
        p.Page = 1
    }
    if p.PageSize == 0 {
        p.PageSize = 50 // Default
    }
    if p.PageSize > 250 {
        p.PageSize = 250 // Max
    }
}

func (p *PaginationParams) ToRepository() repository.Pagination {
    return repository.Pagination{
        Page:     p.Page,
        PageSize: p.PageSize,
    }
}

type PaginatedResponse struct {
    Data   interface{} `json:"data"`
    Total  int64       `json:"total"`
    Limit  int         `json:"limit"`
    Offset int         `json:"offset"`
}
```

**dto/incident_request.go:**
```go
package dto

import (
    "time"
    "github.com/openincident/openincident/internal/models"
)

type CreateIncidentRequest struct {
    Title       string `json:"title" binding:"required,min=1,max=500"`
    Severity    string `json:"severity" binding:"omitempty,oneof=critical high medium low"`
    Description string `json:"description"`
}

type UpdateIncidentRequest struct {
    Status   string `json:"status" binding:"omitempty,oneof=triggered acknowledged resolved canceled"`
    Severity string `json:"severity" binding:"omitempty,oneof=critical high medium low"`
    Summary  string `json:"summary"`
}

type IncidentFilters struct {
    Status       string     `form:"status" binding:"omitempty,oneof=triggered acknowledged resolved canceled"`
    Severity     string     `form:"severity" binding:"omitempty,oneof=critical high medium low"`
    CreatedAfter *time.Time `form:"created_after" time_format:"2006-01-02T15:04:05Z07:00"`
    CreatedBefore *time.Time `form:"created_before" time_format:"2006-01-02T15:04:05Z07:00"`
}

func (f *IncidentFilters) ToRepository() repository.IncidentFilters {
    return repository.IncidentFilters{
        Status:    models.IncidentStatus(f.Status),
        Severity:  models.IncidentSeverity(f.Severity),
        StartDate: f.CreatedAfter,
        EndDate:   f.CreatedBefore,
    }
}
```

**dto/incident_response.go:**
```go
package dto

import (
    "time"
    "github.com/google/uuid"
    "github.com/openincident/openincident/internal/models"
)

type IncidentResponse struct {
    ID               uuid.UUID          `json:"id"`
    IncidentNumber   int                `json:"incident_number"`
    Title            string             `json:"title"`
    Slug             string             `json:"slug"`
    Status           string             `json:"status"`
    Severity         string             `json:"severity"`
    Summary          string             `json:"summary,omitempty"`
    SlackChannel     *SlackChannelInfo  `json:"slack_channel,omitempty"`
    CreatedAt        time.Time          `json:"created_at"`
    TriggeredAt      time.Time          `json:"triggered_at"`
    AcknowledgedAt   *time.Time         `json:"acknowledged_at,omitempty"`
    ResolvedAt       *time.Time         `json:"resolved_at,omitempty"`
    CreatedByType    string             `json:"created_by_type"`
    CreatedByID      string             `json:"created_by_id,omitempty"`
    CommanderID      *uuid.UUID         `json:"commander_id,omitempty"`
}

type SlackChannelInfo struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    URL  string `json:"url"`
}

type IncidentDetailResponse struct {
    IncidentResponse
    Alerts   []AlertSummary        `json:"alerts"`
    Timeline []TimelineEntrySummary `json:"timeline"`
}

type AlertSummary struct {
    ID          uuid.UUID `json:"id"`
    Title       string    `json:"title"`
    Severity    string    `json:"severity"`
    Status      string    `json:"status"`
    ReceivedAt  time.Time `json:"received_at"`
}

type TimelineEntrySummary struct {
    ID        uuid.UUID              `json:"id"`
    Timestamp time.Time              `json:"timestamp"`
    Type      string                 `json:"type"`
    ActorType string                 `json:"actor_type"`
    ActorID   string                 `json:"actor_id,omitempty"`
    Content   map[string]interface{} `json:"content"`
}

func ToIncidentResponse(incident *models.Incident) IncidentResponse {
    resp := IncidentResponse{
        ID:             incident.ID,
        IncidentNumber: incident.IncidentNumber,
        Title:          incident.Title,
        Slug:           incident.Slug,
        Status:         string(incident.Status),
        Severity:       string(incident.Severity),
        Summary:        incident.Summary,
        CreatedAt:      incident.CreatedAt,
        TriggeredAt:    incident.TriggeredAt,
        AcknowledgedAt: incident.AcknowledgedAt,
        ResolvedAt:     incident.ResolvedAt,
        CreatedByType:  incident.CreatedByType,
        CreatedByID:    incident.CreatedByID,
        CommanderID:    incident.CommanderID,
    }

    if incident.SlackChannelID != "" {
        resp.SlackChannel = &SlackChannelInfo{
            ID:   incident.SlackChannelID,
            Name: incident.SlackChannelName,
            URL:  fmt.Sprintf("https://slack.com/app_redirect?channel=%s", incident.SlackChannelID),
        }
    }

    return resp
}
```

**dto/timeline_request.go:**
```go
package dto

type CreateTimelineEntryRequest struct {
    Type    string                 `json:"type" binding:"required,eq=message"`
    Content map[string]interface{} `json:"content" binding:"required"`
}
```

**dto/timeline_response.go:**
```go
package dto

import (
    "time"
    "github.com/google/uuid"
    "github.com/openincident/openincident/internal/models"
)

type TimelineEntryResponse struct {
    ID        uuid.UUID              `json:"id"`
    Timestamp time.Time              `json:"timestamp"`
    Type      string                 `json:"type"`
    ActorType string                 `json:"actor_type"`
    ActorID   string                 `json:"actor_id,omitempty"`
    Content   map[string]interface{} `json:"content"`
}

func ToTimelineEntryResponse(entry *models.TimelineEntry) TimelineEntryResponse {
    return TimelineEntryResponse{
        ID:        entry.ID,
        Timestamp: entry.Timestamp,
        Type:      entry.Type,
        ActorType: entry.ActorType,
        ActorID:   entry.ActorID,
        Content:   entry.Content,
    }
}
```

**Verification:**
```bash
# Code compiles
cd backend && go build ./...

# DTOs can be marshaled/unmarshaled
go test ./internal/api/handlers/dto/...
```

---

### Task 2: Add Request ID Middleware (OI-035 partial)

**Files:**
- `backend/internal/api/middleware/request_id.go` (NEW)

**Implementation:**
```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Check if request ID exists in header (from load balancer)
        requestID := c.GetHeader("X-Request-ID")

        // Generate new ID if not provided
        if requestID == "" {
            requestID = uuid.New().String()
        }

        // Store in context for handlers and logging
        c.Set("request_id", requestID)

        // Include in response headers
        c.Header("X-Request-ID", requestID)

        c.Next()
    }
}
```

**Wire up in routes.go:**
```go
func SetupRoutes(router *gin.Engine, db *gorm.DB) {
    // ... existing setup ...

    // Middleware
    router.Use(middleware.RequestID())  // ADD THIS FIRST
    router.Use(middleware.CORS())
    router.Use(middleware.Recovery())
    router.Use(middleware.Logger())

    // ... rest of routes ...
}
```

**Verification:**
```bash
# Test request ID is added to all responses
curl -i http://localhost:8080/health
# Should see: X-Request-ID: <uuid>

curl -i -H "X-Request-ID: custom-id" http://localhost:8080/health
# Should see: X-Request-ID: custom-id
```

---

### Task 3: Implement GET /api/v1/incidents (OI-028)

**Files:**
- `backend/internal/api/handlers/incidents.go` (NEW)

**Implementation:**
```go
package handlers

import (
    "log/slog"
    "github.com/gin-gonic/gin"
    "github.com/openincident/openincident/internal/api/handlers/dto"
    "github.com/openincident/openincident/internal/services"
)

// ListIncidents handles GET /api/v1/incidents
func ListIncidents(incidentSvc services.IncidentService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Parse query parameters
        var filters dto.IncidentFilters
        if err := c.ShouldBindQuery(&filters); err != nil {
            dto.ValidationError(c, err)
            return
        }

        var pagination dto.PaginationParams
        if err := c.ShouldBindQuery(&pagination); err != nil {
            dto.ValidationError(c, err)
            return
        }
        pagination.Normalize()

        // Fetch incidents from service
        incidents, total, err := incidentSvc.ListIncidents(
            filters.ToRepository(),
            pagination.ToRepository(),
        )
        if err != nil {
            slog.Error("failed to list incidents",
                "error", err,
                "request_id", c.GetString("request_id"),
            )
            dto.InternalError(c, err)
            return
        }

        // Convert to response DTOs
        responses := make([]dto.IncidentResponse, len(incidents))
        for i, incident := range incidents {
            responses[i] = dto.ToIncidentResponse(&incident)
        }

        // Return paginated response
        c.JSON(200, dto.PaginatedResponse{
            Data:   responses,
            Total:  total,
            Limit:  pagination.PageSize,
            Offset: (pagination.Page - 1) * pagination.PageSize,
        })
    }
}
```

**Update service interface:**
```go
// In backend/internal/services/incident_service.go

type IncidentService interface {
    CreateIncidentFromAlert(alert *models.Alert) (*models.Incident, error)
    ShouldCreateIncident(severity models.AlertSeverity) bool
    CreateSlackChannelForIncident(incident *models.Incident, alerts []models.Alert) error

    // ADD THESE:
    ListIncidents(filters repository.IncidentFilters, pagination repository.Pagination) ([]models.Incident, int64, error)
    GetIncident(id uuid.UUID, number int) (*models.Incident, error)
    CreateIncident(req *CreateIncidentParams) (*models.Incident, error)
    UpdateIncident(id uuid.UUID, req *UpdateIncidentParams) (*models.Incident, error)
}

// Add implementation methods in incidentService struct
func (s *incidentService) ListIncidents(filters repository.IncidentFilters, pagination repository.Pagination) ([]models.Incident, int64, error) {
    return s.incidentRepo.List(filters, pagination)
}
```

**Wire up route:**
```go
// In backend/internal/api/routes.go

v1.GET("/incidents", handlers.ListIncidents(incidentSvc))
```

**Verification:**
```bash
# Test list incidents
curl "http://localhost:8080/api/v1/incidents?limit=10&page=1"

# Test with filters
curl "http://localhost:8080/api/v1/incidents?status=triggered&severity=critical"

# Test pagination
curl "http://localhost:8080/api/v1/incidents?limit=5&offset=10"

# Test validation error
curl "http://localhost:8080/api/v1/incidents?status=invalid"
# Should return 400 with field error
```

---

### Task 4: Implement GET /api/v1/incidents/:id (OI-029)

**Files:**
- `backend/internal/api/handlers/incidents.go` (MODIFY)

**Add to handlers/incidents.go:**
```go
// GetIncident handles GET /api/v1/incidents/:id
func GetIncident(incidentSvc services.IncidentService) gin.HandlerFunc {
    return func(c *gin.Context) {
        idParam := c.Param("id")

        // Parse identifier (UUID or incident number)
        uid, num, err := parseIncidentIdentifier(idParam)
        if err != nil {
            dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
                "id": "must be a valid UUID or incident number",
            })
            return
        }

        // Fetch incident
        incident, err := incidentSvc.GetIncident(uid, num)
        if err != nil {
            if _, ok := err.(*repository.NotFoundError); ok {
                dto.NotFound(c, "incident", idParam)
                return
            }
            dto.InternalError(c, err)
            return
        }

        // Fetch linked alerts
        alerts, err := incidentSvc.GetIncidentAlerts(incident.ID)
        if err != nil {
            slog.Error("failed to fetch incident alerts", "error", err)
            alerts = []models.Alert{} // Continue with empty alerts
        }

        // Fetch recent timeline (last 50 by default)
        timeline, _, err := incidentSvc.GetIncidentTimeline(incident.ID, repository.Pagination{
            Page:     1,
            PageSize: 50,
        })
        if err != nil {
            slog.Error("failed to fetch incident timeline", "error", err)
            timeline = []models.TimelineEntry{} // Continue with empty timeline
        }

        // Build response
        resp := dto.IncidentDetailResponse{
            IncidentResponse: dto.ToIncidentResponse(incident),
            Alerts:          make([]dto.AlertSummary, len(alerts)),
            Timeline:        make([]dto.TimelineEntrySummary, len(timeline)),
        }

        for i, alert := range alerts {
            resp.Alerts[i] = dto.AlertSummary{
                ID:         alert.ID,
                Title:      alert.Title,
                Severity:   string(alert.Severity),
                Status:     string(alert.Status),
                ReceivedAt: alert.ReceivedAt,
            }
        }

        for i, entry := range timeline {
            resp.Timeline[i] = dto.TimelineEntrySummary{
                ID:        entry.ID,
                Timestamp: entry.Timestamp,
                Type:      entry.Type,
                ActorType: entry.ActorType,
                ActorID:   entry.ActorID,
                Content:   entry.Content,
            }
        }

        c.JSON(200, resp)
    }
}

// parseIncidentIdentifier parses an incident identifier (UUID or number)
func parseIncidentIdentifier(id string) (uuid.UUID, int, error) {
    // Try parsing as incident number first
    if num, err := strconv.Atoi(id); err == nil {
        return uuid.Nil, num, nil
    }

    // Try parsing as UUID
    if uid, err := uuid.Parse(id); err == nil {
        return uid, 0, nil
    }

    return uuid.Nil, 0, fmt.Errorf("invalid incident identifier")
}
```

**Update service:**
```go
// Add to IncidentService interface and implementation

func (s *incidentService) GetIncident(id uuid.UUID, number int) (*models.Incident, error) {
    if id != uuid.Nil {
        return s.incidentRepo.GetByID(id)
    }
    return s.incidentRepo.GetByNumber(number)
}

func (s *incidentService) GetIncidentAlerts(incidentID uuid.UUID) ([]models.Alert, error) {
    return s.incidentRepo.GetAlerts(incidentID)
}

func (s *incidentService) GetIncidentTimeline(incidentID uuid.UUID, pagination repository.Pagination) ([]models.TimelineEntry, int64, error) {
    return s.timelineRepo.GetByIncidentID(incidentID, pagination)
}
```

**Wire up route:**
```go
v1.GET("/incidents/:id", handlers.GetIncident(incidentSvc))
```

**Verification:**
```bash
# Create an incident first via webhook or manual creation

# Get by UUID
curl "http://localhost:8080/api/v1/incidents/123e4567-e89b-12d3-a456-426614174000"

# Get by incident number
curl "http://localhost:8080/api/v1/incidents/1"

# Test 404
curl "http://localhost:8080/api/v1/incidents/999999"
# Should return 404 with clear message

# Test invalid ID
curl "http://localhost:8080/api/v1/incidents/invalid"
# Should return 400
```

---

### Task 5: Implement POST /api/v1/incidents (OI-030)

**Files:**
- `backend/internal/api/handlers/incidents.go` (MODIFY)
- `backend/internal/services/incident_service.go` (MODIFY)

**Add to handlers/incidents.go:**
```go
// CreateIncident handles POST /api/v1/incidents
func CreateIncident(incidentSvc services.IncidentService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Parse request body
        var req dto.CreateIncidentRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            dto.ValidationError(c, err)
            return
        }

        // Create incident via service
        params := services.CreateIncidentParams{
            Title:       req.Title,
            Severity:    models.IncidentSeverity(req.Severity),
            Description: req.Description,
            CreatedBy:   "user", // For v0.1, hardcoded. Will use auth context in v0.2+
        }

        // Default severity to medium if not specified
        if params.Severity == "" {
            params.Severity = models.IncidentSeverityMedium
        }

        incident, err := incidentSvc.CreateIncident(&params)
        if err != nil {
            slog.Error("failed to create incident",
                "error", err,
                "request_id", c.GetString("request_id"),
            )
            dto.InternalError(c, err)
            return
        }

        // Return created incident
        c.JSON(201, dto.ToIncidentResponse(incident))
    }
}
```

**Add to services/incident_service.go:**
```go
type CreateIncidentParams struct {
    Title       string
    Severity    models.IncidentSeverity
    Description string
    CreatedBy   string // "user", "system", "api"
}

func (s *incidentService) CreateIncident(params *CreateIncidentParams) (*models.Incident, error) {
    // Generate slug
    slug := generateSlug(params.Title)

    // Create incident
    incident := &models.Incident{
        ID:            uuid.New(),
        Title:         params.Title,
        Slug:          slug,
        Status:        models.IncidentStatusTriggered,
        Severity:      params.Severity,
        Summary:       params.Description,
        CreatedByType: "user",
        CreatedByID:   params.CreatedBy,
        TriggeredAt:   time.Now(),
        Labels:        make(models.JSONB),
        CustomFields:  make(models.JSONB),
    }

    // Execute in transaction
    err := s.db.Transaction(func(tx *gorm.DB) error {
        // Create incident
        if err := s.incidentRepo.Create(incident); err != nil {
            return fmt.Errorf("failed to create incident: %w", err)
        }

        // Create timeline entry
        timelineEntry := &models.TimelineEntry{
            ID:         uuid.New(),
            IncidentID: incident.ID,
            Timestamp:  time.Now(),
            Type:       models.TimelineTypeIncidentCreated,
            ActorType:  "user",
            ActorID:    params.CreatedBy,
            Content: models.JSONB{
                "trigger": "manual",
            },
        }

        if err := s.timelineRepo.Create(timelineEntry); err != nil {
            return fmt.Errorf("failed to create timeline entry: %w", err)
        }

        return nil
    })

    if err != nil {
        return nil, err
    }

    // Create Slack channel asynchronously
    if s.chatService != nil {
        go func() {
            if err := s.CreateSlackChannelForIncident(incident, []models.Alert{}); err != nil {
                slog.Error("failed to create slack channel",
                    "incident_id", incident.ID,
                    "error", err)
            }
        }()
    }

    return incident, nil
}
```

**Wire up route:**
```go
v1.POST("/incidents", handlers.CreateIncident(incidentSvc))
```

**Verification:**
```bash
# Create manual incident
curl -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Database slow queries",
    "severity": "high",
    "description": "Users reporting slow page loads"
  }'

# Should return 201 with incident object
# Should create Slack channel asynchronously

# Test validation
curl -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{}'
# Should return 400 with "title: field is required"

# Test invalid severity
curl -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test",
    "severity": "invalid"
  }'
# Should return 400 with severity validation error
```

---

### Task 6: Implement PATCH /api/v1/incidents/:id (OI-031)

**Files:**
- `backend/internal/api/handlers/incidents.go` (MODIFY)
- `backend/internal/services/incident_service.go` (MODIFY)

**Add to handlers/incidents.go:**
```go
// UpdateIncident handles PATCH /api/v1/incidents/:id
func UpdateIncident(incidentSvc services.IncidentService) gin.HandlerFunc {
    return func(c *gin.Context) {
        idParam := c.Param("id")

        // Parse identifier
        uid, num, err := parseIncidentIdentifier(idParam)
        if err != nil {
            dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
                "id": "must be a valid UUID or incident number",
            })
            return
        }

        // Parse request body
        var req dto.UpdateIncidentRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            dto.ValidationError(c, err)
            return
        }

        // Fetch current incident
        incident, err := incidentSvc.GetIncident(uid, num)
        if err != nil {
            if _, ok := err.(*repository.NotFoundError); ok {
                dto.NotFound(c, "incident", idParam)
                return
            }
            dto.InternalError(c, err)
            return
        }

        // Validate status transition if status is being changed
        if req.Status != "" && req.Status != string(incident.Status) {
            requestedStatus := models.IncidentStatus(req.Status)
            if !isValidTransition(incident.Status, requestedStatus) {
                dto.Conflict(c, "Invalid status transition", map[string]interface{}{
                    "current_status":    string(incident.Status),
                    "requested_status":  req.Status,
                    "valid_transitions": getValidTransitions(incident.Status),
                })
                return
            }
        }

        // Update incident
        params := services.UpdateIncidentParams{
            Status:   models.IncidentStatus(req.Status),
            Severity: models.IncidentSeverity(req.Severity),
            Summary:  req.Summary,
            UpdatedBy: "user", // For v0.1, hardcoded
        }

        updatedIncident, err := incidentSvc.UpdateIncident(incident.ID, &params)
        if err != nil {
            dto.InternalError(c, err)
            return
        }

        c.JSON(200, dto.ToIncidentResponse(updatedIncident))
    }
}

// State machine validation
var validTransitions = map[models.IncidentStatus][]models.IncidentStatus{
    models.IncidentStatusTriggered: {
        models.IncidentStatusAcknowledged,
        models.IncidentStatusResolved,
        models.IncidentStatusCanceled,
    },
    models.IncidentStatusAcknowledged: {
        models.IncidentStatusResolved,
    },
    models.IncidentStatusResolved: {},
    models.IncidentStatusCanceled: {},
}

func isValidTransition(current, requested models.IncidentStatus) bool {
    if current == requested {
        return true // No-op
    }

    allowed := validTransitions[current]
    for _, status := range allowed {
        if status == requested {
            return true
        }
    }
    return false
}

func getValidTransitions(current models.IncidentStatus) []string {
    allowed := validTransitions[current]
    transitions := make([]string, len(allowed))
    for i, status := range allowed {
        transitions[i] = string(status)
    }
    return transitions
}
```

**Add to services/incident_service.go:**
```go
type UpdateIncidentParams struct {
    Status    models.IncidentStatus
    Severity  models.IncidentSeverity
    Summary   string
    UpdatedBy string
}

func (s *incidentService) UpdateIncident(id uuid.UUID, params *UpdateIncidentParams) (*models.Incident, error) {
    // Fetch current incident
    incident, err := s.incidentRepo.GetByID(id)
    if err != nil {
        return nil, err
    }

    previousStatus := incident.Status
    previousSeverity := incident.Severity

    // Execute update in transaction
    err = s.db.Transaction(func(tx *gorm.DB) error {
        // Update fields if provided
        if params.Status != "" && params.Status != incident.Status {
            if err := s.incidentRepo.UpdateStatus(id, params.Status); err != nil {
                return err
            }

            // Create timeline entry for status change
            timelineEntry := &models.TimelineEntry{
                ID:         uuid.New(),
                IncidentID: id,
                Timestamp:  time.Now(),
                Type:       models.TimelineTypeStatusChanged,
                ActorType:  "user",
                ActorID:    params.UpdatedBy,
                Content: models.JSONB{
                    "previous_status": string(previousStatus),
                    "new_status":      string(params.Status),
                },
            }
            if err := s.timelineRepo.Create(timelineEntry); err != nil {
                return err
            }
        }

        if params.Severity != "" && params.Severity != incident.Severity {
            incident.Severity = params.Severity

            // Create timeline entry for severity change
            timelineEntry := &models.TimelineEntry{
                ID:         uuid.New(),
                IncidentID: id,
                Timestamp:  time.Now(),
                Type:       models.TimelineTypeSeverityChanged,
                ActorType:  "user",
                ActorID:    params.UpdatedBy,
                Content: models.JSONB{
                    "previous_severity": string(previousSeverity),
                    "new_severity":      string(params.Severity),
                },
            }
            if err := s.timelineRepo.Create(timelineEntry); err != nil {
                return err
            }
        }

        if params.Summary != "" {
            incident.Summary = params.Summary
        }

        // Update incident
        if err := s.incidentRepo.Update(incident); err != nil {
            return err
        }

        return nil
    })

    if err != nil {
        return nil, err
    }

    // Post Slack notification asynchronously (Task 7)
    if params.Status != "" && params.Status != previousStatus && s.chatService != nil {
        go func() {
            if err := s.PostStatusUpdateToSlack(incident, previousStatus, params.Status); err != nil {
                slog.Error("failed to post slack notification", "error", err)
            }
        }()
    }

    // Fetch updated incident
    return s.incidentRepo.GetByID(id)
}
```

**Wire up route:**
```go
v1.PATCH("/incidents/:id", handlers.UpdateIncident(incidentSvc))
```

**Verification:**
```bash
# Acknowledge incident
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"status": "acknowledged"}'

# Should return 200 with updated incident
# Should create timeline entry
# Should post to Slack (Task 7)

# Resolve incident
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'

# Update severity
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"severity": "critical"}'

# Test invalid transition (resolved → acknowledged)
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"status": "acknowledged"}'
# Should return 409 with valid transitions
```

---

### Task 7: Implement Slack Status Update Notifications (OI-034)

**Files:**
- `backend/internal/services/incident_service.go` (MODIFY)
- `backend/internal/services/slack_message_builder.go` (MODIFY)

**Add to slack_message_builder.go:**
```go
// BuildStatusUpdateMessage creates a message for incident status changes
func (b *SlackMessageBuilder) BuildStatusUpdateMessage(
    incident *models.Incident,
    previousStatus models.IncidentStatus,
    newStatus models.IncidentStatus,
) Message {
    // Status emoji mapping
    statusEmoji := map[models.IncidentStatus]string{
        models.IncidentStatusTriggered:    "🔴",
        models.IncidentStatusAcknowledged: "🟡",
        models.IncidentStatusResolved:     "🟢",
        models.IncidentStatusCanceled:     "⚫",
    }

    emoji := statusEmoji[newStatus]
    title := fmt.Sprintf("%s Incident #%d: %s → %s",
        emoji,
        incident.IncidentNumber,
        strings.ToUpper(string(previousStatus)),
        strings.ToUpper(string(newStatus)),
    )

    blocks := []slack.Block{
        slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", title, true, false)),
        slack.NewDividerBlock(),
        slack.NewSectionBlock(
            slack.NewTextBlockObject("mrkdwn",
                fmt.Sprintf("*Incident:* %s\n*Previous Status:* %s\n*New Status:* %s\n*Changed At:* <!date^%d^{date_short_pretty} at {time}|%s>",
                    incident.Title,
                    strings.Title(string(previousStatus)),
                    strings.Title(string(newStatus)),
                    time.Now().Unix(),
                    time.Now().Format("2006-01-02 15:04:05 MST"),
                ),
                nil,
                nil,
            ),
        ),
    }

    // Add specific messaging for terminal states
    if newStatus == models.IncidentStatusResolved {
        blocks = append(blocks,
            slack.NewContextBlock("", slack.NewTextBlockObject("mrkdwn", "✅ This incident has been resolved. Great work team!", false, false)),
        )
    } else if newStatus == models.IncidentStatusCanceled {
        blocks = append(blocks,
            slack.NewContextBlock("", slack.NewTextBlockObject("mrkdwn", "⚠️ This incident has been canceled.", false, false)),
        )
    }

    return Message{
        Text:   title,
        Blocks: convertToInterface(blocks),
    }
}
```

**Add to incident_service.go:**
```go
// PostStatusUpdateToSlack posts a status update message to the incident's Slack channel
func (s *incidentService) PostStatusUpdateToSlack(
    incident *models.Incident,
    previousStatus models.IncidentStatus,
    newStatus models.IncidentStatus,
) error {
    if s.chatService == nil {
        return fmt.Errorf("slack service not configured")
    }

    if incident.SlackChannelID == "" {
        return fmt.Errorf("incident has no slack channel")
    }

    message := s.messageBuilder.BuildStatusUpdateMessage(incident, previousStatus, newStatus)

    _, err := s.chatService.PostMessage(incident.SlackChannelID, message)
    if err != nil {
        return fmt.Errorf("failed to post status update to slack: %w", err)
    }

    slog.Info("posted status update to slack",
        "incident_id", incident.ID,
        "incident_number", incident.IncidentNumber,
        "previous_status", previousStatus,
        "new_status", newStatus,
        "channel_id", incident.SlackChannelID,
    )

    return nil
}
```

**Verification:**
```bash
# Create incident
curl -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{"title": "Test Slack Notifications", "severity": "high"}'

# Wait for Slack channel creation

# Acknowledge incident → Should see message in Slack
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"status": "acknowledged"}'

# Check Slack channel for status update message

# Resolve incident → Should see message in Slack
curl -X PATCH http://localhost:8080/api/v1/incidents/1 \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'

# Check Slack channel for resolution message with checkmark
```

---

### Task 8: Implement GET /api/v1/incidents/:id/timeline (OI-032)

**Files:**
- `backend/internal/api/handlers/timeline.go` (NEW)

**Create timeline.go:**
```go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/openincident/openincident/internal/api/handlers/dto"
    "github.com/openincident/openincident/internal/services"
)

// GetIncidentTimeline handles GET /api/v1/incidents/:id/timeline
func GetIncidentTimeline(incidentSvc services.IncidentService) gin.HandlerFunc {
    return func(c *gin.Context) {
        idParam := c.Param("id")

        // Parse identifier
        uid, num, err := parseIncidentIdentifier(idParam)
        if err != nil {
            dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
                "id": "must be a valid UUID or incident number",
            })
            return
        }

        // Fetch incident to verify existence
        incident, err := incidentSvc.GetIncident(uid, num)
        if err != nil {
            if _, ok := err.(*repository.NotFoundError); ok {
                dto.NotFound(c, "incident", idParam)
                return
            }
            dto.InternalError(c, err)
            return
        }

        // Parse pagination
        var pagination dto.PaginationParams
        if err := c.ShouldBindQuery(&pagination); err != nil {
            dto.ValidationError(c, err)
            return
        }
        pagination.Normalize()

        // Fetch timeline entries
        entries, total, err := incidentSvc.GetIncidentTimeline(incident.ID, pagination.ToRepository())
        if err != nil {
            dto.InternalError(c, err)
            return
        }

        // Convert to response DTOs
        responses := make([]dto.TimelineEntryResponse, len(entries))
        for i, entry := range entries {
            responses[i] = dto.ToTimelineEntryResponse(&entry)
        }

        c.JSON(200, dto.PaginatedResponse{
            Data:   responses,
            Total:  total,
            Limit:  pagination.PageSize,
            Offset: (pagination.Page - 1) * pagination.PageSize,
        })
    }
}
```

**Wire up route:**
```go
v1.GET("/incidents/:id/timeline", handlers.GetIncidentTimeline(incidentSvc))
```

**Verification:**
```bash
# Get timeline for incident
curl "http://localhost:8080/api/v1/incidents/1/timeline?limit=50"

# Should return chronological timeline entries (timestamp ASC)

# Test pagination
curl "http://localhost:8080/api/v1/incidents/1/timeline?limit=10&page=2"

# Test 404 for non-existent incident
curl "http://localhost:8080/api/v1/incidents/999999/timeline"
# Should return 404
```

---

### Task 9: Implement POST /api/v1/incidents/:id/timeline (OI-033)

**Files:**
- `backend/internal/api/handlers/timeline.go` (MODIFY)

**Add to timeline.go:**
```go
// CreateTimelineEntry handles POST /api/v1/incidents/:id/timeline
func CreateTimelineEntry(incidentSvc services.IncidentService) gin.HandlerFunc {
    return func(c *gin.Context) {
        idParam := c.Param("id")

        // Parse identifier
        uid, num, err := parseIncidentIdentifier(idParam)
        if err != nil {
            dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
                "id": "must be a valid UUID or incident number",
            })
            return
        }

        // Fetch incident
        incident, err := incidentSvc.GetIncident(uid, num)
        if err != nil {
            if _, ok := err.(*repository.NotFoundError); ok {
                dto.NotFound(c, "incident", idParam)
                return
            }
            dto.InternalError(c, err)
            return
        }

        // Parse request
        var req dto.CreateTimelineEntryRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            dto.ValidationError(c, err)
            return
        }

        // Validate type (only "message" allowed for manual entries)
        if req.Type != "message" {
            dto.BadRequest(c, "Invalid timeline entry type", map[string]interface{}{
                "type": "manual entries must have type='message'",
            })
            return
        }

        // Create entry
        params := services.CreateTimelineEntryParams{
            IncidentID: incident.ID,
            Type:       req.Type,
            Content:    req.Content,
            ActorType:  "user",
            ActorID:    "anonymous", // For v0.1. Will use auth context in v0.2+
        }

        entry, err := incidentSvc.CreateTimelineEntry(&params)
        if err != nil {
            dto.InternalError(c, err)
            return
        }

        c.JSON(201, dto.ToTimelineEntryResponse(entry))
    }
}
```

**Add to services/incident_service.go:**
```go
type CreateTimelineEntryParams struct {
    IncidentID uuid.UUID
    Type       string
    Content    models.JSONB
    ActorType  string
    ActorID    string
}

func (s *incidentService) CreateTimelineEntry(params *CreateTimelineEntryParams) (*models.TimelineEntry, error) {
    entry := &models.TimelineEntry{
        ID:         uuid.New(),
        IncidentID: params.IncidentID,
        Timestamp:  time.Now(),
        Type:       params.Type,
        ActorType:  params.ActorType,
        ActorID:    params.ActorID,
        Content:    params.Content,
    }

    if err := s.timelineRepo.Create(entry); err != nil {
        return nil, err
    }

    return entry, nil
}
```

**Wire up route:**
```go
v1.POST("/incidents/:id/timeline", handlers.CreateTimelineEntry(incidentSvc))
```

**Verification:**
```bash
# Add user note to incident
curl -X POST http://localhost:8080/api/v1/incidents/1/timeline \
  -H "Content-Type: application/json" \
  -d '{
    "type": "message",
    "content": {
      "text": "Starting investigation. Checking database logs."
    }
  }'

# Should return 201 with entry
# Timestamp should be server-generated

# Test validation - invalid type
curl -X POST http://localhost:8080/api/v1/incidents/1/timeline \
  -H "Content-Type: application/json" \
  -d '{
    "type": "status_changed",
    "content": {}
  }'
# Should return 400 - only "message" allowed for manual entries

# Test validation - missing fields
curl -X POST http://localhost:8080/api/v1/incidents/1/timeline \
  -H "Content-Type: application/json" \
  -d '{}'
# Should return 400 with field errors
```

---

### Task 10: Update Routes and Wire Everything Up

**Files:**
- `backend/internal/api/routes.go` (MODIFY)

**Update routes.go:**
```go
func SetupRoutes(router *gin.Engine, db *gorm.DB) {
    // ... existing repository and service initialization ...

    // Middleware
    router.Use(middleware.RequestID())  // NEW - must be first
    router.Use(middleware.CORS())
    router.Use(middleware.Recovery())
    router.Use(middleware.Logger())

    // Health check endpoints
    router.GET("/health", handlers.Health(db))
    router.GET("/ready", handlers.Ready(db))

    // API v1 routes
    v1 := router.Group("/api/v1")
    {
        // Webhooks
        webhooks := v1.Group("/webhooks")
        {
            webhooks.POST("/prometheus", handlers.PrometheusWebhook(alertSvc))
            webhooks.POST("/grafana", func(c *gin.Context) {
                c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
            })
        }

        // Incidents - REPLACE STUBS
        v1.GET("/incidents", handlers.ListIncidents(incidentSvc))
        v1.GET("/incidents/:id", handlers.GetIncident(incidentSvc))
        v1.POST("/incidents", handlers.CreateIncident(incidentSvc))
        v1.PATCH("/incidents/:id", handlers.UpdateIncident(incidentSvc))
        v1.GET("/incidents/:id/timeline", handlers.GetIncidentTimeline(incidentSvc))
        v1.POST("/incidents/:id/timeline", handlers.CreateTimelineEntry(incidentSvc))

        // Alerts (existing stub - no changes)
        v1.GET("/alerts", func(c *gin.Context) {
            c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
        })
    }
}
```

**Verification:**
```bash
# Verify all routes registered
curl http://localhost:8080/health
# Should return 200

# List all routes
go run cmd/openincident/main.go
# Should see in logs:
# [GIN-debug] GET    /health
# [GIN-debug] GET    /ready
# [GIN-debug] POST   /api/v1/webhooks/prometheus
# [GIN-debug] GET    /api/v1/incidents
# [GIN-debug] GET    /api/v1/incidents/:id
# [GIN-debug] POST   /api/v1/incidents
# [GIN-debug] PATCH  /api/v1/incidents/:id
# [GIN-debug] GET    /api/v1/incidents/:id/timeline
# [GIN-debug] POST   /api/v1/incidents/:id/timeline
```

---

### Task 11: Code Quality - Format, Lint, Test

**Commands:**
```bash
# Format all code
cd backend && gofmt -w .

# Run linter
cd backend && go vet ./...

# Run tests (if any exist)
cd backend && go test ./...

# Build to verify no compilation errors
cd backend && go build ./...
```

**Verification:**
```bash
# Should have no errors
# All new code formatted
# No vet warnings
```

---

### Task 12: End-to-End Testing

**Test Scenarios:**

**Scenario 1: Manual Incident Creation Flow**
```bash
# 1. Create incident
INCIDENT=$(curl -s -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "E2E Test: Manual Incident",
    "severity": "high",
    "description": "Testing manual incident creation"
  }' | jq -r '.incident_number')

echo "Created incident #$INCIDENT"

# 2. Wait for Slack channel creation
sleep 5

# 3. Get incident details
curl -s "http://localhost:8080/api/v1/incidents/$INCIDENT" | jq

# 4. Add user note
curl -s -X POST "http://localhost:8080/api/v1/incidents/$INCIDENT/timeline" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "message",
    "content": {"text": "Starting investigation"}
  }' | jq

# 5. Acknowledge
curl -s -X PATCH "http://localhost:8080/api/v1/incidents/$INCIDENT" \
  -H "Content-Type: application/json" \
  -d '{"status": "acknowledged"}' | jq

# 6. Check Slack for status update

# 7. Resolve
curl -s -X PATCH "http://localhost:8080/api/v1/incidents/$INCIDENT" \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}' | jq

# 8. Get timeline
curl -s "http://localhost:8080/api/v1/incidents/$INCIDENT/timeline" | jq

# Expected timeline:
# - incident_created (trigger: manual)
# - message (user note)
# - status_changed (triggered → acknowledged)
# - status_changed (acknowledged → resolved)
```

**Scenario 2: Alert-Triggered Incident Flow**
```bash
# 1. Send Prometheus alert (critical)
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d @test_alert_critical.json

# 2. List incidents - should see new incident
curl -s "http://localhost:8080/api/v1/incidents?status=triggered" | jq

# 3. Get incident by number
INCIDENT=$(curl -s "http://localhost:8080/api/v1/incidents?limit=1" | jq -r '.data[0].incident_number')

curl -s "http://localhost:8080/api/v1/incidents/$INCIDENT" | jq

# Should have:
# - status: triggered
# - created_by_type: system
# - alerts array with 1 alert
# - timeline with incident_created + alert_linked

# 4. Update to resolved
curl -X PATCH "http://localhost:8080/api/v1/incidents/$INCIDENT" \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'

# 5. Check Slack for status update
```

**Scenario 3: Invalid State Transitions**
```bash
# Create and resolve incident
INCIDENT=$(curl -s -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{"title": "Test", "severity": "low"}' | jq -r '.incident_number')

curl -X PATCH "http://localhost:8080/api/v1/incidents/$INCIDENT" \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'

# Try to acknowledge a resolved incident (invalid)
curl -i -X PATCH "http://localhost:8080/api/v1/incidents/$INCIDENT" \
  -H "Content-Type: application/json" \
  -d '{"status": "acknowledged"}'

# Should return 409 Conflict with:
# {
#   "error": {
#     "code": "conflict",
#     "message": "Invalid status transition",
#     "details": {
#       "current_status": "resolved",
#       "requested_status": "acknowledged",
#       "valid_transitions": []
#     }
#   }
# }
```

**Scenario 4: Pagination and Filtering**
```bash
# Create multiple incidents with different statuses
for i in {1..10}; do
  curl -s -X POST http://localhost:8080/api/v1/incidents \
    -H "Content-Type: application/json" \
    -d "{\"title\": \"Test $i\", \"severity\": \"low\"}" > /dev/null
done

# List all
curl -s "http://localhost:8080/api/v1/incidents?limit=5" | jq '.total'
# Should return 10

# Filter by status
curl -s "http://localhost:8080/api/v1/incidents?status=triggered" | jq '.data | length'

# Pagination
curl -s "http://localhost:8080/api/v1/incidents?limit=3&page=2" | jq '.offset'
# Should return 3
```

---

### Task 13: Commit All Changes

**Commit Strategy:**
Group related changes into logical commits following conventional commit format.

```bash
# 1. DTOs and error handling
git add backend/internal/api/handlers/dto/
git commit -m "feat(api): add DTOs and standardized error responses

- Add request/response DTOs for incidents and timeline
- Implement pagination DTOs
- Create error response builders with field-level validation
- Add request ID tracking for debugging

Implements OI-035 (API error handling)"

# 2. Request ID middleware
git add backend/internal/api/middleware/request_id.go
git commit -m "feat(middleware): add request ID middleware

- Generate unique request ID per request
- Support X-Request-ID header from load balancers
- Include request ID in all error responses

Part of OI-035"

# 3. Incident handlers
git add backend/internal/api/handlers/incidents.go
git commit -m "feat(api): implement incident CRUD endpoints

- GET /api/v1/incidents - list with filtering and pagination
- GET /api/v1/incidents/:id - get single incident with details
- POST /api/v1/incidents - create manual incident
- PATCH /api/v1/incidents/:id - update incident with validation
- Support both UUID and incident number lookups
- Validate status transitions with state machine

Implements OI-028, OI-029, OI-030, OI-031"

# 4. Timeline handlers
git add backend/internal/api/handlers/timeline.go
git commit -m "feat(api): implement timeline endpoints

- GET /api/v1/incidents/:id/timeline - list timeline entries
- POST /api/v1/incidents/:id/timeline - add user notes
- Paginated responses
- Chronological ordering (timestamp ASC)

Implements OI-032, OI-033"

# 5. Incident service updates
git add backend/internal/services/incident_service.go
git commit -m "feat(services): extend incident service for API operations

- Add ListIncidents, GetIncident, CreateIncident, UpdateIncident
- Add CreateTimelineEntry for user notes
- Implement state validation and timeline tracking
- Add PostStatusUpdateToSlack for notifications

Supports OI-028 through OI-034"

# 6. Slack status notifications
git add backend/internal/services/slack_message_builder.go
git commit -m "feat(slack): add status update notifications

- Build rich Block Kit messages for status changes
- Include emoji indicators for status
- Add special messaging for terminal states (resolved, canceled)
- Post asynchronously to avoid blocking API responses

Implements OI-034"

# 7. Routes update
git add backend/internal/api/routes.go
git commit -m "feat(api): wire up incident API endpoints

- Register all incident and timeline routes
- Add RequestID middleware as first middleware
- Remove placeholder stubs

Completes Epic 005 wiring"

# 8. Format and cleanup
gofmt -w .
git add -u
git commit -m "style: format code with gofmt"
```

**Verification:**
```bash
# View commits
git log --oneline --author="$(git config user.name)" | head -10

# Should see 8 commits for Epic 005
```

---

## 4. Success Criteria Verification

After completing all tasks, verify:

### ✅ Functional Requirements
- [ ] All incident endpoints return correct HTTP status codes
- [ ] Validation errors include field-level details
- [ ] Status transitions are validated with clear error messages
- [ ] Request ID is included in all responses
- [ ] Slack notifications posted on status changes
- [ ] Timeline entries created for all state changes
- [ ] Pagination works correctly
- [ ] Filtering works correctly
- [ ] Both UUID and incident number lookups work

### ✅ Error Handling
- [ ] 400 Bad Request for validation errors
- [ ] 404 Not Found with resource type
- [ ] 409 Conflict for invalid state transitions
- [ ] 500 Internal Server Error logs full error, returns safe message
- [ ] All responses include request_id

### ✅ Code Quality
- [ ] All code formatted with gofmt
- [ ] No go vet warnings
- [ ] Clear separation of concerns (handlers, services, repositories)
- [ ] Consistent error handling pattern
- [ ] Proper logging with structured fields

### ✅ Documentation
- [ ] Implementation plan complete
- [ ] ADRs documented
- [ ] Commit messages follow conventional commits
- [ ] Code comments explain business logic

---

## 5. Technical Debt and Future Work

**Deferred to v0.2:**
- Authentication and authorization (currently hardcoded "user" / "anonymous")
- Commander assignment
- Responder management
- Custom fields and labels editing
- Bulk operations
- CSV export
- Incident search (full-text)

**Deferred to v0.3:**
- Incident templates
- Auto-escalation based on time thresholds
- SLA tracking
- Incident metrics and dashboards

---

## 6. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Breaking existing webhook flow | Low | High | Thorough testing of existing Prometheus webhook after changes |
| State machine bugs (invalid transitions) | Medium | Medium | Explicit unit tests for all transition paths |
| Slack notification failures blocking API | Low | High | Already async with goroutines |
| Performance issues with large timelines | Medium | Low | Already paginated in repository |

---

## 7. Estimated Effort

| Task | Estimated Time | Dependencies |
|------|---------------|--------------|
| Task 1: DTOs | 2 hours | None |
| Task 2: Request ID | 30 minutes | None |
| Task 3: List Incidents | 1.5 hours | Task 1, 2 |
| Task 4: Get Incident | 1.5 hours | Task 1, 2 |
| Task 5: Create Incident | 2 hours | Task 1, 2 |
| Task 6: Update Incident | 3 hours | Task 1, 2, 5 |
| Task 7: Slack Notifications | 1.5 hours | Task 6 |
| Task 8: Get Timeline | 1 hour | Task 1, 2 |
| Task 9: Create Timeline Entry | 1 hour | Task 1, 2, 8 |
| Task 10: Wire Routes | 30 minutes | All above |
| Task 11: Code Quality | 1 hour | All above |
| Task 12: E2E Testing | 2 hours | All above |
| Task 13: Commits | 30 minutes | All above |
| **Total** | **18 hours** | |

---

## 8. References

- Epic 003: Prometheus Webhook Integration (handler pattern reference)
- Epic 004: Slack Integration (async pattern, message builder reference)
- CLAUDE.md: Project context and architecture principles
- ARCHITECTURE.md: Full API specification
- ADR-008: Request/Response DTO Pattern
- ADR-009: Centralized Error Handling
- ADR-010: Incident ID Resolution
- ADR-011: Status Transition Validation

---

*This plan follows the same subagent-driven workflow as Epic 002 and Epic 004: implement → spec review → fix → code quality review → complete.*

**Status:** Ready for execution
**Next Step:** Begin Task 1 - Create DTOs and Validation Utilities

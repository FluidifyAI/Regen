// Package enterprise defines the extension points that the proprietary
// enterprise tier implements. The OSS build wires in no-op stubs so that
// the server runs identically whether or not enterprise code is present.
//
// Enterprise repo usage:
//
//	import "github.com/FluidifyAI/Regen/backend/enterprise"
//
//	hooks := enterprise.Hooks{
//	    RBAC:      myrbac.NewProvider(db),
//	    Audit:     myaudit.NewExporter(cfg),
//	    SCIM:      myscim.NewHandler(db),
//	    Retention: myretention.NewWorker(db),
//	}
//	// pass hooks to serve.go → routes.go + worker.StartAll
package enterprise

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/FluidifyAI/Regen/backend/ui"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ── RBAC ─────────────────────────────────────────────────────────────────────

// RBACProvider enforces role-based access control on API routes.
// The no-op implementation allows every request through — OSS has a
// single implicit "admin" role for all authenticated users.
type RBACProvider interface {
	// Middleware returns a Gin handler that enforces the given permission.
	// resource examples: "incident", "schedule", "user"
	// action  examples: "read", "write", "delete"
	Middleware(resource, action string) gin.HandlerFunc
}

// ── Audit ─────────────────────────────────────────────────────────────────────

// AuditExporter records a structured event log suitable for SOC2 audit trails.
// Called from the API middleware layer after each significant request.
type AuditExporter interface {
	// Export persists an audit event. Implementations must be non-blocking
	// (queue internally) — the caller does not wait for the write to complete.
	Export(ctx context.Context, event AuditEvent)
}

// AuditEvent represents a single auditable action in the system.
type AuditEvent struct {
	Timestamp    time.Time // server-generated, always UTC
	ActorID      string    // user ID or "system"
	ActorType    string    // "user" | "system" | "api_key"
	Action       string    // dot-separated: "incident.created", "user.login", etc.
	ResourceType string    // "incident" | "user" | "schedule" | ...
	ResourceID   string    // UUID of the affected resource
	IPAddress    string    // from X-Forwarded-For or RemoteAddr
	UserAgent    string
	StatusCode   int            // HTTP response status
	Metadata     map[string]any // action-specific extra fields
}

// ── SCIM ──────────────────────────────────────────────────────────────────────

// SCIMHandler mounts SCIM 2.0 endpoints for automated user provisioning
// (Okta, Azure AD, OneLogin, etc.). The no-op stub returns 501 on all routes
// so that misconfigured identity providers get a clear error, not a 404.
type SCIMHandler interface {
	// RegisterRoutes mounts the SCIM endpoints on the provided router group.
	// The group is already prefixed with /scim/v2 by the caller.
	RegisterRoutes(group *gin.RouterGroup)
}

// ── Retention ────────────────────────────────────────────────────────────────

// RetentionEnforcer runs the data retention policy background worker.
// Policies define how long incidents, timeline entries, and audit logs
// are kept before being anonymised or deleted.
type RetentionEnforcer interface {
	// Start launches the worker. Must be non-blocking (runs its own goroutine).
	// The context is cancelled on server shutdown.
	Start(ctx context.Context, db *gorm.DB)
}

// ── UI ────────────────────────────────────────────────────────────────────────

// UIProvider supplies the embedded frontend filesystem served by the API server.
// The OSS no-op returns the OSS build; the Pro binary returns a Pro-built FS
// that includes all Pro-only pages and components.
type UIProvider interface {
	// FS returns the embedded frontend as an fs.FS rooted at dist/, or nil when
	// no frontend has been built (the API still works, just no SPA).
	FS() fs.FS
}

// ── Custom Fields ─────────────────────────────────────────────────────────────

// CustomFieldsHandler mounts custom field definition endpoints.
// The no-op stub returns 402 on all routes — custom fields require a Pro licence.
type CustomFieldsHandler interface {
	RegisterRoutes(group *gin.RouterGroup, db *gorm.DB)
}

// ── Cost Tracking ─────────────────────────────────────────────────────────────

// UsageEvent records one AI call for cost accounting.
type UsageEvent struct {
	Operation        string    // "summarize" | "postmortem" | "handoff" | "enhance_postmortem" | "enhance_draft" | "answer_question"
	Model            string    // e.g. "gpt-4o", "claude-3-5-sonnet"
	PromptTokens     int
	CompletionTokens int
	OccurredAt       time.Time // caller must supply time.Now().UTC()
}

// CostSummary aggregates AI spend across all recorded usage events.
type CostSummary struct {
	TotalUSD        float64            `json:"total_usd"`
	CurrentMonthUSD float64            `json:"current_month_usd"`
	ByOperation     map[string]float64 `json:"by_operation"`
}

// CostTracker records AI usage events and surfaces cost summaries.
// The no-op implementation records nothing and returns zero cost — OSS behaviour is unchanged.
type CostTracker interface {
	// RecordUsage stores the event and returns the estimated USD cost.
	// Returns 0 if the model has no configured pricing. Must never block the caller
	// on failure — implementations must handle DB errors internally.
	RecordUsage(ctx context.Context, event UsageEvent) (costUSD float64, err error)

	// GetSummary returns aggregated spend totals.
	GetSummary(ctx context.Context) (CostSummary, error)

	// RegisterRoutes mounts the cost config and summary API endpoints.
	// The group is already prefixed with /api/v1/ai/cost by the caller.
	RegisterRoutes(group *gin.RouterGroup, db *gorm.DB)
}

// ── Hooks — the single struct threaded through the app ───────────────────────

// Hooks is passed from serve.go to routes.go and worker.StartAll.
// All fields default to their no-op stubs via NewNoOp().
type Hooks struct {
	RBAC         RBACProvider
	Audit        AuditExporter
	SCIM         SCIMHandler
	Retention    RetentionEnforcer
	CustomFields CustomFieldsHandler
	UI           UIProvider
	CostTracker  CostTracker
}

// NewNoOp returns Hooks with all no-op stubs — the default for the OSS build.
func NewNoOp() Hooks {
	return Hooks{
		RBAC:         noopRBAC{},
		Audit:        noopAudit{},
		SCIM:         noopSCIM{},
		Retention:    noopRetention{},
		CustomFields: noopCustomFields{},
		UI:           noopUI{},
		CostTracker:  noopCostTracker{},
	}
}

// ── No-op implementations ─────────────────────────────────────────────────────

// noopRBAC allows every request. OSS treats all authenticated users as admins.
type noopRBAC struct{}

func (noopRBAC) Middleware(_, _ string) gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// noopAudit discards all events. OSS has no audit log export.
type noopAudit struct{}

func (noopAudit) Export(_ context.Context, _ AuditEvent) {}

// noopSCIM returns 501 Not Implemented on all routes so IdPs get a clear signal.
type noopSCIM struct{}

func (noopSCIM) RegisterRoutes(group *gin.RouterGroup) {
	group.Any("/*path", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"detail": "SCIM provisioning requires a Fluidify Regen Enterprise license.",
		})
	})
}

// noopRetention is a no-op — OSS keeps data indefinitely.
type noopRetention struct{}

func (noopRetention) Start(_ context.Context, _ *gorm.DB) {}

// noopUI serves the OSS-built frontend. When no frontend has been compiled,
// FS() returns nil and the router silently skips static file serving.
type noopUI struct{}

func (noopUI) FS() fs.FS { return ui.FS() }

// noopCustomFields returns 402 on all routes — custom fields are a Pro feature.
type noopCustomFields struct{}

func (noopCustomFields) RegisterRoutes(group *gin.RouterGroup, _ *gorm.DB) {
	group.Any("/*path", func(c *gin.Context) {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": "custom fields require a Fluidify Regen Pro licence",
		})
	})
}

// noopCostTracker records nothing and returns zero cost.
// OSS handlers always emit cost_usd: 0 — no feature flag needed.
type noopCostTracker struct{}

func (noopCostTracker) RecordUsage(_ context.Context, _ UsageEvent) (float64, error) {
	return 0, nil
}

func (noopCostTracker) GetSummary(_ context.Context) (CostSummary, error) {
	return CostSummary{}, nil
}

func (noopCostTracker) RegisterRoutes(group *gin.RouterGroup, _ *gorm.DB) {
	group.Any("/*path", func(c *gin.Context) {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": "AI cost tracking requires a Fluidify Regen Pro licence",
		})
	})
}

// Package enterprise defines the extension points that the proprietary
// enterprise tier implements. The OSS build wires in no-op stubs so that
// the server runs identically whether or not enterprise code is present.
//
// Enterprise repo usage:
//
//	import "github.com/FluidifyAI/Regen/backend/internal/enterprise"
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
	"net/http"
	"time"

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

// ── Hooks — the single struct threaded through the app ───────────────────────

// Hooks is passed from serve.go to routes.go and worker.StartAll.
// All fields default to their no-op stubs via NewNoOp().
type Hooks struct {
	RBAC      RBACProvider
	Audit     AuditExporter
	SCIM      SCIMHandler
	Retention RetentionEnforcer
}

// NewNoOp returns Hooks with all no-op stubs — the default for the OSS build.
func NewNoOp() Hooks {
	return Hooks{
		RBAC:      noopRBAC{},
		Audit:     noopAudit{},
		SCIM:      noopSCIM{},
		Retention: noopRetention{},
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

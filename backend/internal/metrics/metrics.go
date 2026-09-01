package metrics

import (
	"strconv"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/observability"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gorm.io/gorm"
)

var (
	// Event counters — incremented in real-time by services
	WebhooksProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "regen_webhooks_processed_total",
			Help: "Total webhooks processed by source and status (success/error)",
		},
		[]string{"source", "status"},
	)

	AlertsReceivedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "regen_alerts_received_total",
			Help: "Total alerts received by source",
		},
		[]string{"source"},
	)

	IncidentsCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "regen_incidents_created_total",
			Help: "Total incidents created by severity and trigger (alert/manual)",
		},
		[]string{"severity", "trigger"},
	)

	EscalationsTriggeredTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "regen_escalations_triggered_total",
			Help: "Total escalation tiers triggered",
		},
	)

	WorkerJobsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "regen_worker_jobs_processed_total",
			Help: "Total background jobs processed by type",
		},
		[]string{"job_type"},
	)

	WorkerJobsFailedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "regen_worker_jobs_failed_total",
			Help: "Total background jobs failed by type",
		},
		[]string{"job_type"},
	)

	// WorkerJobDurationSeconds is the Duration leg of RED for every
	// background worker job (WorkerJobsProcessedTotal/FailedTotal are
	// Rate/Errors). Each worker tick already opens its own root span via
	// observability.StartWorkerTick, so callers can attach a trace exemplar
	// via observability.ObserveWithTraceExemplar for free.
	WorkerJobDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "regen_worker_job_duration_seconds",
			Help:    "Duration of one background worker job execution by job type.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"job_type"},
	)

	// EscalationDispatchFailedTotal and EscalationDispatchDurationSeconds
	// are the Errors/Duration legs of RED for the "escalation dispatch"
	// critical path — resolving a tier's on-call targets and sending their
	// notifications (EscalationsTriggeredTotal above is Rate, but only ever
	// counted on success; before REG-13 a failed dispatch was logged and
	// silently swallowed by the caller with zero metric visibility).
	//
	// No trace exemplar on this histogram: dispatch happens deep inside
	// EscalationEngine, which has no ctx parameter on its public interface
	// today — threading one through would cascade across every method and
	// every test double, the same interface-fan-out cost documented for
	// REG-157/REG-160. Out of scope here; Duration is still fully present
	// without it.
	EscalationDispatchFailedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "regen_escalation_dispatch_failed_total",
			Help: "Total escalation tier dispatch attempts that failed (target resolution or state persistence).",
		},
	)

	EscalationDispatchDurationSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "regen_escalation_dispatch_duration_seconds",
			Help:    "Duration of one escalation tier dispatch: resolving on-call targets through sending their notifications.",
			Buckets: prometheus.DefBuckets,
		},
	)

	// NotificationsSentTotal and NotificationSendDurationSeconds are the
	// Rate+Errors / Duration legs of RED for the "notification send" critical
	// path, which had no metrics at all before REG-13. Every delivery
	// channel shares these same two metrics, keyed by the "channel" label —
	// W5's new channels (email, sms, voice) reuse them with a new channel
	// value rather than needing metrics of their own.
	//
	// No trace exemplar here either, for the same reason as
	// EscalationDispatchDurationSeconds: the send call sites this wires into
	// (EscalationWorker.SendEscalationDM) have no ctx/span available without
	// the same interface-fan-out threading deferred above.
	NotificationsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "regen_notifications_sent_total",
			Help: "Total notification send attempts by channel and status (success/error/skipped).",
		},
		[]string{"channel", "status"},
	)

	NotificationSendDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "regen_notification_send_duration_seconds",
			Help:    "Duration of one notification send attempt by channel.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"channel"},
	)

	// HTTP request metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// Business metrics
	incidentsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "incidents_total",
			Help: "Total number of incidents by status",
		},
		[]string{"status"},
	)

	incidentsBySeverity = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "incidents_by_severity",
			Help: "Total number of incidents by severity",
		},
		[]string{"severity"},
	)

	alertsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alerts_total",
			Help: "Total number of alerts by status",
		},
		[]string{"status"},
	)

	// Database connection pool metrics
	dbConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_open",
			Help: "Number of open database connections",
		},
	)

	dbConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_in_use",
			Help: "Number of database connections in use",
		},
	)

	dbConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)
)

// unmatchedRouteLabel stands in for c.FullPath() on requests gin never
// matched to a registered route (404s, and any other path that reaches this
// middleware without a route template). Using the raw incoming URL as a
// label value instead — as this code did before REG-13 — is unbounded
// cardinality: every distinct 404'd path, including ones an unauthenticated
// caller controls, mints a brand new Prometheus time series that is never
// cleaned up.
const unmatchedRouteLabel = "unmatched"

// Middleware returns a Gin middleware that instruments HTTP requests
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = unmatchedRouteLabel
		}

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		observability.ObserveWithTraceExemplar(
			c.Request.Context(),
			httpRequestDuration.WithLabelValues(c.Request.Method, path, status),
			duration,
		)
	}
}

// UpdateBusinessMetrics updates incident and alert metrics from the database
func UpdateBusinessMetrics(db *gorm.DB) {
	// Update incident metrics by status
	var incidentsByStatus []struct {
		Status string
		Count  int64
	}
	db.Table("incidents").Select("status, count(*) as count").Group("status").Scan(&incidentsByStatus)
	for _, stat := range incidentsByStatus {
		incidentsTotal.WithLabelValues(stat.Status).Set(float64(stat.Count))
	}

	// Update incident metrics by severity
	var incidentsBySev []struct {
		Severity string
		Count    int64
	}
	db.Table("incidents").Select("severity, count(*) as count").Group("severity").Scan(&incidentsBySev)
	for _, stat := range incidentsBySev {
		incidentsBySeverity.WithLabelValues(stat.Severity).Set(float64(stat.Count))
	}

	// Update alert metrics by status
	var alertsByStatus []struct {
		Status string
		Count  int64
	}
	db.Table("alerts").Select("status, count(*) as count").Group("status").Scan(&alertsByStatus)
	for _, stat := range alertsByStatus {
		alertsTotal.WithLabelValues(stat.Status).Set(float64(stat.Count))
	}

	// Update database connection pool metrics
	sqlDB, err := db.DB()
	if err == nil {
		stats := sqlDB.Stats()
		dbConnectionsOpen.Set(float64(stats.OpenConnections))
		dbConnectionsInUse.Set(float64(stats.InUse))
		dbConnectionsIdle.Set(float64(stats.Idle))
	}
}

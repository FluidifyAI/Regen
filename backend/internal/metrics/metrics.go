package metrics

import (
	"strconv"
	"time"

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
			path = c.Request.URL.Path
		}

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path, status).Observe(duration)
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

package api

// TestMetricsEndpoint_NegotiatesOpenMetrics proves REG-13's critical fix: the
// /metrics handler must be built with promhttp.HandlerOpts{EnableOpenMetrics:
// true}, or exemplars are silently unservable no matter how carefully the
// Observe side (metrics.Middleware, the worker histograms) attaches them —
// OpenMetrics is the only exposition format that carries exemplars at all,
// and client_golang only negotiates it when this option is set (confirmed
// against promhttp's own source: EnableOpenMetrics controls which Negotiate
// function content negotiation goes through).
//
// This test builds the exact handler construction routes.go uses, in
// isolation, rather than exercising the full SetupRoutes dependency graph
// (db, config, SAML, Teams, ...) that isn't relevant to this one negotiation
// behavior.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMetricsEndpoint_NegotiatesOpenMetrics_WhenRequested(t *testing.T) {
	router := gin.New()
	router.GET("/metrics", gin.WrapH(metricsHandler()))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept", "application/openmetrics-text; version=1.0.0")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/openmetrics-text") {
		t.Errorf("Content-Type = %q, want application/openmetrics-text — Prometheus (scraping with "+
			"--enable-feature=exemplar-storage) will never receive exemplars otherwise", ct)
	}
}

func TestMetricsEndpoint_StillServesPlainTextForOlderScrapers(t *testing.T) {
	router := gin.New()
	router.GET("/metrics", gin.WrapH(metricsHandler()))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	// No Accept header — a scraper that doesn't ask for OpenMetrics must
	// still get a response (backward compatibility with older Prometheus/
	// any other consumer of this endpoint).
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Error("expected a non-empty metrics body")
	}
}

package observability

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/propagation"
)

// excludedFromTracing lists route templates that are high-volume and carry no
// useful trace information: liveness/readiness probes and metrics scraping.
// Keyed by the matched route template, not the raw path.
var excludedFromTracing = map[string]bool{
	"/health":  true,
	"/ready":   true,
	"/metrics": true,
}

// newPropagator builds the W3C-standard propagator (traceparent/tracestate,
// plus baggage) that lets an inbound trace continue into Regen and lets
// Regen's own outbound calls carry its trace context onward.
func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// GinMiddleware opens a root span per inbound HTTP request, extracting W3C
// traceparent/tracestate from the request so a caller's trace continues into
// Regen rather than starting fresh. Health, readiness, and metrics endpoints
// are excluded — see excludedFromTracing.
//
// Span names use the matched route template (e.g. "/api/v1/incidents/:id"),
// never the raw request path: otelgin's default naming is c.FullPath() when
// no SpanNameFormatter is supplied, falling back to a bounded
// "HTTP <method> route not found" for unmatched (404) requests — exactly the
// behavior wanted here, so no override is needed.
//
// The W3C propagator is passed explicitly rather than relying on whatever
// happens to be the process-global default at call time — deterministic in
// production, and lets tests inject their own TracerProvider via opts without
// touching global otel state (which is a shared, order-dependent singleton).
func GinMiddleware(opts ...otelgin.Option) gin.HandlerFunc {
	base := []otelgin.Option{
		otelgin.WithPropagators(newPropagator()),
		otelgin.WithGinFilter(func(c *gin.Context) bool {
			return !excludedFromTracing[c.FullPath()]
		}),
	}
	return otelgin.Middleware("regen", append(base, opts...)...)
}

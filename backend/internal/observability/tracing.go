// Package observability stands up the tracing foundation for Regen: an
// OpenTelemetry TracerProvider exporting via OTLP/gRPC, disabled by default so
// self-hosters are never forced into running a collector.
package observability

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// Config carries the resource attributes InitTracer stamps onto every span.
// Exporter configuration itself comes from the standard OTEL_EXPORTER_OTLP_*
// env vars, read by the SDK — not threaded through here.
type Config struct {
	ServiceVersion string
	Environment    string
}

// InitTracer stands up tracing and installs it as the global TracerProvider.
//
// Disabled by default: when OTEL_EXPORTER_OTLP_ENDPOINT is not set, InitTracer
// installs a no-op TracerProvider (zero overhead) and returns immediately.
// Self-hosters must never be forced into running a collector to start Regen.
//
// The returned shutdown func flushes and closes the exporter; call it during
// the existing SIGTERM drain so no spans are lost on exit.
func InitTracer(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		// OTel's own contract: an error handed here must never propagate as a
		// panic or a returned error — log once and keep running degraded.
		slog.Error("observability: opentelemetry error", "error", err)
	}))

	// Set globally regardless of enabled/disabled state below: outbound HTTP
	// client instrumentation (W0.6) relies on this ambient default so Regen's
	// own calls carry trace context onward, independent of whether tracing
	// itself is exporting anywhere.
	otel.SetTextMapPropagator(newPropagator())

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		slog.Error("observability: failed to build resource, tracing disabled", "error", err)
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	// otlptracegrpc.New reads OTEL_EXPORTER_OTLP_ENDPOINT and
	// OTEL_EXPORTER_OTLP_HEADERS itself when no explicit option overrides them.
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		slog.Error("observability: failed to create OTLP exporter, tracing disabled", "error", err)
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// newResource builds the resource stamped onto every span: a fixed
// service.name of "regen", the caller-supplied version and environment, with
// OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES from the environment applied
// last so an operator can override the default when they explicitly set them.
func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("regen"),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
		resource.WithFromEnv(),
	)
}

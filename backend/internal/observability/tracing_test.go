package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// clearOTelEnv resets the OTel env vars this package reads and restores them
// after the test, so tests don't leak state into each other or a real
// developer environment.
func clearOTelEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_HEADERS",
		"OTEL_SERVICE_NAME",
	} {
		old, had := os.LookupEnv(key)
		os.Unsetenv(key)
		t.Cleanup(func() {
			if had {
				os.Setenv(key, old)
			}
		})
	}
}

func TestInitTracer_DisabledByDefaultProducesNoopSpans(t *testing.T) {
	clearOTelEnv(t)

	shutdown, err := InitTracer(context.Background(), Config{
		ServiceVersion: "test",
		Environment:    "development",
	})
	if err != nil {
		t.Fatalf("InitTracer returned error with no endpoint configured: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("shutdown returned error: %v", err)
		}
	}()

	_, span := otel.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	if span.SpanContext().IsValid() {
		t.Fatal("expected a no-op span (invalid SpanContext) when no OTLP endpoint is configured")
	}
}

func TestNewResource_SetsServiceNameVersionAndEnvironment(t *testing.T) {
	clearOTelEnv(t)

	res, err := newResource(context.Background(), Config{
		ServiceVersion: "1.2.3",
		Environment:    "production",
	})
	if err != nil {
		t.Fatalf("newResource returned error: %v", err)
	}

	set := res.Set()
	wantAttrs := map[attribute.Key]string{
		semconv.ServiceNameKey:           "regen",
		semconv.ServiceVersionKey:        "1.2.3",
		semconv.DeploymentEnvironmentKey: "production",
	}
	for key, want := range wantAttrs {
		got, ok := set.Value(key)
		if !ok {
			t.Errorf("resource missing attribute %q", key)
			continue
		}
		if got.AsString() != want {
			t.Errorf("resource attribute %q = %q, want %q", key, got.AsString(), want)
		}
	}
}

func TestNewResource_OTELServiceNameEnvOverridesDefault(t *testing.T) {
	clearOTelEnv(t)
	os.Setenv("OTEL_SERVICE_NAME", "regen-worker")

	res, err := newResource(context.Background(), Config{
		ServiceVersion: "1.2.3",
		Environment:    "production",
	})
	if err != nil {
		t.Fatalf("newResource returned error: %v", err)
	}

	got, ok := res.Set().Value(semconv.ServiceNameKey)
	if !ok {
		t.Fatal("resource missing service.name attribute")
	}
	if got.AsString() != "regen-worker" {
		t.Errorf("service.name = %q, want %q (OTEL_SERVICE_NAME should override the \"regen\" default)", got.AsString(), "regen-worker")
	}
}

func TestInitTracer_InstallsRealSDKTracerProviderWhenEnabled(t *testing.T) {
	clearOTelEnv(t)
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	shutdown, err := InitTracer(context.Background(), Config{
		ServiceVersion: "1.2.3",
		Environment:    "production",
	})
	if err != nil {
		t.Fatalf("InitTracer returned error: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_ = shutdown(ctx)
	}()

	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Fatalf("expected *sdktrace.TracerProvider to be installed globally, got %T", otel.GetTracerProvider())
	}
}

func TestInitTracer_ShutdownDoesNotPanicWhenCollectorUnreachable(t *testing.T) {
	clearOTelEnv(t)
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	shutdown, err := InitTracer(context.Background(), Config{
		ServiceVersion: "test",
		Environment:    "development",
	})
	if err != nil {
		t.Fatalf("InitTracer returned error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = shutdown(ctx) // error is acceptable (no live collector); panic is not
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not return in time — should degrade gracefully, not hang")
	}
}

func TestOTelErrorHandlerLogsInsteadOfCrashing(t *testing.T) {
	clearOTelEnv(t)

	shutdown, err := InitTracer(context.Background(), Config{
		ServiceVersion: "test",
		Environment:    "development",
	})
	if err != nil {
		t.Fatalf("InitTracer returned error: %v", err)
	}
	defer shutdown(context.Background())

	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prevLogger)

	// This must not panic the test process — that's the behavior under test.
	otel.Handle(errors.New("simulated exporter failure"))

	if !bytes.Contains(buf.Bytes(), []byte("simulated exporter failure")) {
		t.Fatalf("expected the OTel error handler to log the error via slog, got: %q", buf.String())
	}
}

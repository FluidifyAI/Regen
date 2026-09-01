package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
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

func TestInitTracer_InstallsW3CPropagatorGlobally(t *testing.T) {
	clearOTelEnv(t)

	// Deliberately not saved/restored: otel's global propagator is a shared,
	// order-dependent singleton (Get returns the live delegating wrapper, not
	// a snapshot), so Set-it-back-in-cleanup is unreliable — and also not how
	// production works. InitTracer sets this once, for the life of the
	// process; this test only asserts the forward behavior after that call.
	shutdown, err := InitTracer(context.Background(), Config{
		ServiceVersion: "test",
		Environment:    "development",
	})
	if err != nil {
		t.Fatalf("InitTracer returned error: %v", err)
	}
	defer shutdown(context.Background())

	// Real behavior, not a type assertion on the propagator's internals:
	// extract a W3C traceparent header and confirm the trace actually
	// continues with the embedded trace ID.
	const wantTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	carrier := propagation.MapCarrier{
		"traceparent": "00-" + wantTraceID + "-00f067aa0ba902b7-01",
	}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

	sc := oteltrace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatal("expected a valid SpanContext extracted from the traceparent header — propagator not installed?")
	}
	if got := sc.TraceID().String(); got != wantTraceID {
		t.Errorf("extracted TraceID = %q, want %q", got, wantTraceID)
	}
}

func TestHasURLScheme(t *testing.T) {
	cases := map[string]bool{
		"http://collector:4317":  true,
		"https://collector:4318": true,
		"collector:4317":         false,
		"localhost:4317":         false,
		"":                       false,
	}
	for endpoint, want := range cases {
		if got := hasURLScheme(endpoint); got != want {
			t.Errorf("hasURLScheme(%q) = %v, want %v", endpoint, got, want)
		}
	}
}

func TestInitTracer_WarnsWhenEndpointMissingScheme(t *testing.T) {
	clearOTelEnv(t)
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prevLogger)

	shutdown, err := InitTracer(context.Background(), Config{ServiceVersion: "test", Environment: "development"})
	if err != nil {
		t.Fatalf("InitTracer returned error: %v", err)
	}
	defer shutdown(context.Background())

	if !bytes.Contains(buf.Bytes(), []byte("localhost:4317")) {
		t.Errorf("expected a warning naming the malformed endpoint value, got: %q", buf.String())
	}
}

func TestInitTracer_NoWarningWhenEndpointHasScheme(t *testing.T) {
	clearOTelEnv(t)
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")

	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prevLogger)

	shutdown, err := InitTracer(context.Background(), Config{ServiceVersion: "test", Environment: "development"})
	if err != nil {
		t.Fatalf("InitTracer returned error: %v", err)
	}
	defer shutdown(context.Background())

	if bytes.Contains(buf.Bytes(), []byte("missing a URL scheme")) {
		t.Errorf("did not expect a scheme warning for a well-formed endpoint, got: %q", buf.String())
	}
}

// TestInitTracer_ConnectsToBareHostPortEndpoint is the real proof: before this
// fix, a bare host:port endpoint produced an empty gRPC target and no
// connection was ever attempted (delegating_resolver: invalid target address
// "": missing address). A real TCP listener receiving an actual connection
// attempt proves the target is no longer empty — not just that a log line
// changed.
func TestInitTracer_ConnectsToBareHostPortEndpoint(t *testing.T) {
	clearOTelEnv(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer ln.Close()

	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
			accepted <- struct{}{}
		}
	}()

	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", ln.Addr().String()) // bare host:port — no scheme

	shutdown, err := InitTracer(context.Background(), Config{ServiceVersion: "test", Environment: "development"})
	if err != nil {
		t.Fatalf("InitTracer returned error: %v", err)
	}
	defer shutdown(context.Background())

	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		t.Fatalf("expected *sdktrace.TracerProvider to be installed, got %T", otel.GetTracerProvider())
	}
	_, span := tp.Tracer("test").Start(context.Background(), "op")
	span.End()

	// ForceFlush blocks for its own timeout regardless of outcome (our fake
	// listener isn't a real gRPC server, so the handshake never completes) —
	// short timeout since the proof we need (a connection reaching the
	// listener) happens almost immediately, well before any flush deadline.
	flushCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = tp.ForceFlush(flushCtx) // triggers the actual dial; error (e.g. handshake) is fine, we only care that a connection was attempted

	select {
	case <-accepted:
		// A real TCP connection reached our listener — the endpoint was
		// correctly resolved to a non-empty target.
	case <-time.After(1 * time.Second):
		t.Fatal("no connection was ever attempted at the bare host:port endpoint — normalization did not fix the empty-target bug")
	}
}

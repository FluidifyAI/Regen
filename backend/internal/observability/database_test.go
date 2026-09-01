package observability

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	gormtracing "gorm.io/plugin/opentelemetry/tracing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func clearSQLStatementEnv(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv("OTEL_SQL_STATEMENT_ENABLED")
	os.Unsetenv("OTEL_SQL_STATEMENT_ENABLED")
	t.Cleanup(func() {
		if had {
			os.Setenv("OTEL_SQL_STATEMENT_ENABLED", old)
		} else {
			os.Unsetenv("OTEL_SQL_STATEMENT_ENABLED")
		}
	})
}

func TestSQLStatementRecordingEnabled_DefaultsToFalse(t *testing.T) {
	clearSQLStatementEnv(t)
	if SQLStatementRecordingEnabled() {
		t.Error("expected false with OTEL_SQL_STATEMENT_ENABLED unset")
	}
}

func TestSQLStatementRecordingEnabled_TrueWhenExplicitlySet(t *testing.T) {
	clearSQLStatementEnv(t)
	os.Setenv("OTEL_SQL_STATEMENT_ENABLED", "true")
	if !SQLStatementRecordingEnabled() {
		t.Error("expected true with OTEL_SQL_STATEMENT_ENABLED=true")
	}
}

// testWidget is a trivial model — enough to prove a real INSERT produces a
// correctly attributed child span, without needing the full incident schema.
type testWidget struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

// newTracedTestDB wires the real GORMTracingOptions policy but always injects
// an explicit, isolated TracerProvider (tp) rather than relying on the
// process-global otel.SetTracerProvider — that global is a shared,
// order-dependent singleton (see REG-7), so tests must never depend on it.
func newTracedTestDB(t *testing.T, tp trace.TracerProvider) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&testWidget{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	opts := append(GORMTracingOptions(), gormtracing.WithTracerProvider(tp))
	if err := db.Use(gormtracing.NewPlugin(opts...)); err != nil {
		t.Fatalf("register tracing plugin: %v", err)
	}
	return db
}

func TestGORMTracingOptions_ProducesChildSpanForWrite(t *testing.T) {
	clearSQLStatementEnv(t)

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	db := newTracedTestDB(t, tp)

	ctx, parentSpan := tp.Tracer("test").Start(context.Background(), "incident-create")
	if err := db.WithContext(ctx).Create(&testWidget{Name: "test"}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	parentSpan.End()

	spans := sr.Ended()
	if len(spans) < 2 {
		t.Fatalf("expected at least 2 spans (parent + DB write child), got %d", len(spans))
	}

	var dbSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Parent().SpanID() == parentSpan.SpanContext().SpanID() {
			dbSpan = s
		}
	}
	if dbSpan == nil {
		t.Fatal("no span was recorded as a child of the parent span — DB write did not produce a child span")
	}
	if dbSpan.SpanContext().TraceID() != parentSpan.SpanContext().TraceID() {
		t.Error("DB span has a different trace ID than the parent — not actually correlated")
	}
}

func TestGORMTracingOptions_RedactsQueryTextByDefault(t *testing.T) {
	clearSQLStatementEnv(t)

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	db := newTracedTestDB(t, tp)
	ctx := context.Background()

	const secretName = "super-secret-alert-label-value"
	if err := db.WithContext(ctx).Create(&testWidget{Name: secretName}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	for _, s := range sr.Ended() {
		for _, attr := range s.Attributes() {
			if strings.Contains(attr.Value.Emit(), secretName) {
				t.Fatalf("found interpolated value %q in span attribute %s=%s — query text must be redacted by default",
					secretName, attr.Key, attr.Value.Emit())
			}
		}
	}
}

func TestGORMTracingOptions_RecordsQueryStructureWhenEnabled(t *testing.T) {
	clearSQLStatementEnv(t)
	os.Setenv("OTEL_SQL_STATEMENT_ENABLED", "true")

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	db := newTracedTestDB(t, tp)
	ctx := context.Background()

	const secretName = "super-secret-alert-label-value"
	if err := db.WithContext(ctx).Create(&testWidget{Name: secretName}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var sawQueryText bool
	for _, s := range sr.Ended() {
		for _, attr := range s.Attributes() {
			if string(attr.Key) == "db.query.text" {
				sawQueryText = true
				if strings.Contains(attr.Value.AsString(), secretName) {
					t.Fatalf("query text contains the interpolated value %q — WithoutQueryVariables must always redact values, even when statement text is enabled: %s",
						secretName, attr.Value.AsString())
				}
			}
		}
	}
	if !sawQueryText {
		t.Fatal("expected a db.query.text attribute when OTEL_SQL_STATEMENT_ENABLED=true")
	}
}

func TestRedisTracingOptions_RedactsCommandArgumentsByDefault(t *testing.T) {
	clearSQLStatementEnv(t)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	if err := InstrumentRedis(client, redisotel.WithTracerProvider(tp)); err != nil {
		t.Fatalf("InstrumentRedis: %v", err)
	}

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	const secretValue = "super-secret-alert-payload"
	if err := client.Set(ctx, "alert:123", secretValue, 0).Err(); err != nil {
		t.Fatalf("redis SET: %v", err)
	}
	span.End()

	for _, s := range sr.Ended() {
		for _, attr := range s.Attributes() {
			if strings.Contains(attr.Value.Emit(), secretValue) {
				t.Fatalf("found command argument %q in span attribute %s — redis command args must be redacted by default",
					secretValue, attr.Key)
			}
		}
	}
}

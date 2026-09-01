package observability

import (
	"os"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	gormtracing "gorm.io/plugin/opentelemetry/tracing"
)

// SQLStatementRecordingEnabled reports whether DB/Redis instrumentation
// should record query/command text (structure — table names, operation) on
// spans. Interpolated values are never recorded regardless of this setting;
// only the text/structure toggle is gated here.
//
// Off by default: alert payloads and incident data can carry PII, and query
// text is a data-egress path with the same exposure as a trace export or an
// LLM call — cheap to redact from day one, expensive to retrofit.
func SQLStatementRecordingEnabled() bool {
	return os.Getenv("OTEL_SQL_STATEMENT_ENABLED") == "true"
}

// GORMTracingOptions returns gorm tracing plugin options enforcing this
// package's query-privacy policy. WithoutQueryVariables is always applied —
// interpolated values are never recorded, regardless of the env var — and
// the query text itself is additionally blanked unless explicitly enabled.
func GORMTracingOptions() []gormtracing.Option {
	opts := []gormtracing.Option{gormtracing.WithoutQueryVariables()}
	if !SQLStatementRecordingEnabled() {
		// The plugin always sets a db.query.text attribute; there is no
		// native way to omit it entirely, so blank it via the formatter hook.
		opts = append(opts, gormtracing.WithQueryFormatter(func(string) string { return "" }))
	}
	return opts
}

// InstrumentRedis wraps client with tracing per this package's query-privacy
// policy. redisotel defaults to recording raw command arguments
// (dbStmtEnabled: true) — this flips that default off unless explicitly
// enabled. Extra opts are appended after the policy default, so callers
// (tests) can override things like the TracerProvider.
func InstrumentRedis(client redis.UniversalClient, opts ...redisotel.TracingOption) error {
	base := append([]redisotel.TracingOption{redisotel.WithDBStatement(SQLStatementRecordingEnabled())}, opts...)
	return redisotel.InstrumentTracing(client, base...)
}

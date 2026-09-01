package llm

import "context"

// CallMeta carries the caller-supplied metadata a Complete call gets tagged
// with on its span, when present: AgentName ("Incident Summarizer",
// "Post-Mortem Drafter", ...) and IncidentID, when the call is scoped to a
// specific incident.
//
// Client.Complete's signature intentionally does not carry these — adding
// parameters there would force every provider implementation and every test
// double to change for metadata only the caller (internal/services.aiService)
// actually has. Passing it via context instead means callers set it, and the
// instrumentation decorator reads it, with zero changes to the Client
// interface or its three provider implementations.
type CallMeta struct {
	AgentName  string
	IncidentID string
}

type callMetaContextKey struct{}

// WithCallMeta returns a copy of ctx carrying meta, for the instrumented
// client to read via CallMetaFromContext when it builds a completion's span.
func WithCallMeta(ctx context.Context, meta CallMeta) context.Context {
	return context.WithValue(ctx, callMetaContextKey{}, meta)
}

// CallMetaFromContext returns the CallMeta set by WithCallMeta, or the zero
// value (both fields empty) if none was set — the instrumented client treats
// a zero value as "omit these attributes", not an error.
func CallMetaFromContext(ctx context.Context) CallMeta {
	meta, _ := ctx.Value(callMetaContextKey{}).(CallMeta)
	return meta
}

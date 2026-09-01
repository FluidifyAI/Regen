package observability

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPClientOption configures InstrumentedHTTPClient. A thin wrapper around
// otelhttp.Option so this package's public API doesn't leak otelhttp as a
// required import at every call site — most callers need only the
// (client, peerService) pair.
type HTTPClientOption = otelhttp.Option

// WithHTTPClientTracerProvider overrides the TracerProvider used for a single
// instrumented client. Production code never needs this — otelhttp defaults
// to the global TracerProvider InitTracer installs. Tests use it to inject an
// isolated provider rather than touching global otel state (see REG-7).
func WithHTTPClientTracerProvider(tp trace.TracerProvider) HTTPClientOption {
	return otelhttp.WithTracerProvider(tp)
}

// InstrumentedHTTPClient wraps client's transport with otelhttp, tagging every
// span with peer.service so "Slack was slow" becomes a measurement instead of
// a hunch, and so Teams's two OAuth-scoped clients (Graph API, Bot Framework)
// never get conflated in a trace.
//
// client may be nil, in which case a fresh *http.Client is used. When client
// is non-nil, its existing Transport (e.g. an oauth2.Transport from
// clientcredentials.Config.Client, or a library's own auth-injecting
// RoundTripper) is preserved as the base — instrumentation wraps it, it never
// replaces it, so auth continues to work exactly as before. Timeout,
// CheckRedirect, and Jar are carried over unchanged.
//
// otelhttp.NewTransport never records request/response bodies or headers as
// span data unless otelhttp.WithMessageEvents is explicitly passed, which
// this function does not do — satisfies the "no auth headers, tokens, or
// request bodies on spans" requirement by construction, not by convention.
func InstrumentedHTTPClient(client *http.Client, peerService string, opts ...HTTPClientOption) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	base := append([]otelhttp.Option{
		otelhttp.WithSpanOptions(trace.WithAttributes(semconv.PeerService(peerService))),
	}, opts...)

	return &http.Client{
		Transport:     otelhttp.NewTransport(client.Transport, base...),
		Timeout:       client.Timeout,
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
	}
}

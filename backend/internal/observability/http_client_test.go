package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestInstrumentedHTTPClient_ProducesChildSpanUnderCallersTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	client := InstrumentedHTTPClient(nil, "test-service", WithHTTPClientTracerProvider(tp))

	ctx, parentSpan := tp.Tracer("test").Start(context.Background(), "caller")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	parentSpan.End()

	spans := sr.Ended()
	var httpSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Parent().SpanID() == parentSpan.SpanContext().SpanID() {
			httpSpan = s
		}
	}
	if httpSpan == nil {
		t.Fatalf("no span was recorded as a child of the caller's span — outbound call did not thread ctx into the instrumented transport; spans seen: %d", len(spans))
	}
	if httpSpan.SpanContext().TraceID() != parentSpan.SpanContext().TraceID() {
		t.Error("outbound HTTP span has a different trace ID than the caller — not actually correlated")
	}
}

func TestInstrumentedHTTPClient_TagsPeerService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	client := InstrumentedHTTPClient(nil, "slack", WithHTTPClientTracerProvider(tp))

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	found := false
	for _, attr := range spans[0].Attributes() {
		if string(attr.Key) == "peer.service" && attr.Value.AsString() == "slack" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected peer.service=slack attribute, got attributes: %v", spans[0].Attributes())
	}
}

func TestInstrumentedHTTPClient_DistinctPeerServicesForDistinctClients(t *testing.T) {
	// Mirrors the real Teams shape: two clients hitting the same conceptual
	// integration but different OAuth-scoped endpoints, tagged distinctly so
	// a Teams auth failure doesn't get conflated between Graph API and Bot
	// Framework.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	graphClient := InstrumentedHTTPClient(nil, "msgraph", WithHTTPClientTracerProvider(tp))
	botfwClient := InstrumentedHTTPClient(nil, "botframework", WithHTTPClientTracerProvider(tp))

	for _, c := range []*http.Client{graphClient, botfwClient} {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
	}

	spans := sr.Ended()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	peerServices := make(map[string]bool)
	for _, s := range spans {
		for _, attr := range s.Attributes() {
			if string(attr.Key) == "peer.service" {
				peerServices[attr.Value.AsString()] = true
			}
		}
	}
	if !peerServices["msgraph"] || !peerServices["botframework"] {
		t.Errorf("expected distinct peer.service values msgraph and botframework, got: %v", peerServices)
	}
}

func TestInstrumentedHTTPClient_PreservesExistingTransport(t *testing.T) {
	// Mirrors wrapping an oauth2-managed client (Teams) or a library-supplied
	// client (Firebase): the existing RoundTripper (here, one that injects a
	// header) must still run — instrumentation wraps it, never replaces it.
	var sawCustomHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawCustomHeader = r.Header.Get("X-Custom-Auth") == "secret-token"
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	base := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Custom-Auth", "secret-token")
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	client := InstrumentedHTTPClient(base, "test-service", WithHTTPClientTracerProvider(tp))

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if !sawCustomHeader {
		t.Error("expected the wrapped transport's auth header to still be applied — instrumentation must wrap, not replace, an existing transport")
	}
	if len(sr.Ended()) != 1 {
		t.Errorf("expected the wrapped call to still be traced, got %d spans", len(sr.Ended()))
	}
}

func TestInstrumentedHTTPClient_PreservesTimeout(t *testing.T) {
	base := &http.Client{Timeout: 42 * time.Second}
	client := InstrumentedHTTPClient(base, "test-service")
	if client.Timeout != 42*time.Second {
		t.Errorf("Timeout = %v, want 42s (must be preserved from the base client)", client.Timeout)
	}
}

func TestInstrumentedHTTPClient_NeverRecordsRequestBodyOrHeaders(t *testing.T) {
	// The ticket's own PII requirement: no auth headers, tokens, or request
	// bodies on spans. otelhttp only records these as span *events* when
	// WithMessageEvents is explicitly requested — asserting here that no
	// such events appear pins that behavior against a future regression
	// (e.g. someone adding WithMessageEvents to chase a debugging need).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	client := InstrumentedHTTPClient(nil, "test-service", WithHTTPClientTracerProvider(tp))

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if len(spans[0].Events()) != 0 {
		t.Errorf("expected no span events (message bodies/headers), got %d: %v", len(spans[0].Events()), spans[0].Events())
	}
	for _, attr := range spans[0].Attributes() {
		if attr.Value.Emit() == "Bearer super-secret-token" || string(attr.Key) == "http.request.header.authorization" {
			t.Fatalf("found the auth header value on a span attribute: %s=%s", attr.Key, attr.Value.Emit())
		}
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestInstrumentedHTTPClient_RetriesProduceSiblingSpansNotOneMerged(t *testing.T) {
	// The ticket's own criterion: "Retries appear as sibling spans, not one
	// merged span — retry storms must be visible." otelhttp opens a fresh
	// span per RoundTrip call, so a caller's own retry loop (calling Do()
	// again) naturally produces N independent spans rather than one span
	// with N attempts folded in — verified here, not just asserted.
	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	client := InstrumentedHTTPClient(nil, "test-service", WithHTTPClientTracerProvider(tp))

	ctx, parentSpan := tp.Tracer("test").Start(context.Background(), "caller-with-retry")
	var lastStatus int
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("attempt %d failed: %v", i, err)
		}
		lastStatus = resp.StatusCode
		resp.Body.Close()
		if lastStatus == http.StatusOK {
			break
		}
	}
	parentSpan.End()

	if lastStatus != http.StatusOK {
		t.Fatalf("test setup bug: expected the final attempt to succeed, got %d", lastStatus)
	}

	var childSpans []sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.Parent().SpanID() == parentSpan.SpanContext().SpanID() {
			childSpans = append(childSpans, s)
		}
	}
	if len(childSpans) != 3 {
		t.Fatalf("expected 3 sibling spans (one per retry attempt), got %d — a merged single span would hide the retry storm", len(childSpans))
	}

	statusCodes := make([]int64, 0, 3)
	for _, s := range childSpans {
		for _, attr := range s.Attributes() {
			if string(attr.Key) == "http.response.status_code" {
				statusCodes = append(statusCodes, attr.Value.AsInt64())
			}
		}
	}
	if len(statusCodes) != 3 {
		t.Fatalf("expected each of the 3 spans to carry its own status code, got %d status attributes: %v", len(statusCodes), statusCodes)
	}
}

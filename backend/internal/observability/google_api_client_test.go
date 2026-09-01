package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/api/option"
)

func TestInstrumentedGoogleAPIClient_ProducesChildSpan(t *testing.T) {
	// A request that reaches our test server at all proves the resulting
	// client is real and usable — the actual regression this test guards
	// against is passing option.WithHTTPClient to the Google API transport
	// helpers, which (per the library's own source) returns that client
	// completely bypassing auth construction. Using base as the transport
	// argument, not as WithHTTPClient, is what this function must get right.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	client, err := InstrumentedGoogleAPIClient(context.Background(), "fcm",
		[]option.ClientOption{option.WithoutAuthentication()},
		WithHTTPClientTracerProvider(tp))
	if err != nil {
		t.Fatalf("InstrumentedGoogleAPIClient: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d — instrumentation was not applied to the auth-wrapped transport", len(spans))
	}
	var gotPeerService bool
	for _, attr := range spans[0].Attributes() {
		if string(attr.Key) == "peer.service" && attr.Value.AsString() == "fcm" {
			gotPeerService = true
		}
	}
	if !gotPeerService {
		t.Error("expected peer.service=fcm on the span")
	}
}

func TestInstrumentedGoogleAPIClient_RejectsWithHTTPClientOption(t *testing.T) {
	// option.WithHTTPClient passed into the auth opts would silently bypass
	// auth construction entirely (confirmed against the library source —
	// transport/http.NewTransport itself rejects it with an error). Assert
	// that error surfaces rather than silently producing an unauthenticated
	// client — this is the exact mistake this helper exists to prevent.
	_, err := InstrumentedGoogleAPIClient(context.Background(), "fcm",
		[]option.ClientOption{option.WithHTTPClient(&http.Client{})})
	if err == nil {
		t.Fatal("expected an error when WithHTTPClient is passed in authOpts, got nil")
	}
}

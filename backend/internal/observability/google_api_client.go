package observability

import (
	"context"
	"net/http"

	"google.golang.org/api/option"
	googlehttp "google.golang.org/api/transport/http"
)

// InstrumentedGoogleAPIClient builds an *http.Client for a Google API client
// library (e.g. Firebase's Admin SDK) that is both authenticated per authOpts
// and instrumented with otelhttp.
//
// This exists because google.golang.org/api's own option.WithHTTPClient is
// unsafe to combine with auth options: per the library's source
// (transport/http/dial.go), when settings.HTTPClient is set, NewClient
// returns that client verbatim, skipping its own auth-transport construction
// entirely. Passing an instrumented *http.Client via option.WithHTTPClient
// alongside, say, option.WithAuthCredentialsJSON would silently produce a
// client with zero authentication — every request would fail, and nothing
// about that failure would say "you passed the wrong option."
//
// The correct composition (verified against transport/http.NewTransport's
// source) is to pass the instrumented RoundTripper as the *base* transport
// argument, not as an option — NewTransport then wraps it with the real
// oauth2.Transport built from authOpts, so the resulting client is both
// correctly authenticated and traced. NewTransport itself rejects
// option.WithHTTPClient in authOpts (returns an error) rather than silently
// misbehaving, which InstrumentedGoogleAPIClient surfaces to the caller.
func InstrumentedGoogleAPIClient(ctx context.Context, peerService string, authOpts []option.ClientOption, opts ...HTTPClientOption) (*http.Client, error) {
	instrumented := InstrumentedHTTPClient(nil, peerService, opts...)

	authedTransport, err := googlehttp.NewTransport(ctx, instrumented.Transport, authOpts...)
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: authedTransport}, nil
}

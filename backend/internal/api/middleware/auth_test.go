package middleware

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestIsSecureRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want bool
	}{
		{
			name: "plain HTTP — no TLS, no proxy header",
			req:  &http.Request{Header: http.Header{}},
			want: false,
		},
		{
			name: "native TLS (r.TLS set)",
			req:  &http.Request{Header: http.Header{}, TLS: &tls.ConnectionState{}},
			want: true,
		},
		{
			name: "behind HTTPS reverse proxy (X-Forwarded-Proto: https)",
			req:  &http.Request{Header: http.Header{"X-Forwarded-Proto": []string{"https"}}},
			want: true,
		},
		{
			name: "X-Forwarded-Proto header is HTTP",
			req:  &http.Request{Header: http.Header{"X-Forwarded-Proto": []string{"http"}}},
			want: false,
		},
		{
			name: "X-Forwarded-Proto header is HTTPS mixed-case",
			req:  &http.Request{Header: http.Header{"X-Forwarded-Proto": []string{"HTTPS"}}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSecureRequest(tt.req); got != tt.want {
				t.Errorf("IsSecureRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

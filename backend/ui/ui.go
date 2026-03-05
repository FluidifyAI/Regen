// Package ui embeds the pre-built React frontend and exposes it as an
// http.FileSystem so the Gin router can serve it from the same origin as
// the API (eliminating all CORS configuration for self-hosted deployments).
//
// # Development workflow (no embedding needed)
//
// Run the Vite dev server separately:
//
//	cd frontend && npm run dev
//
// Vite proxies /api → localhost:8080 automatically (see vite.config.ts).
// The backend serves only the API; the browser fetches everything from Vite.
//
// # Production / Docker workflow
//
// The multi-stage Dockerfile builds the frontend first, copies the output
// into this directory, then compiles the Go binary.  The embed directive
// captures whatever is in dist/ at compile time.
//
// If dist/ contains only .gitkeep (i.e. no real frontend build), Files()
// returns nil and the router skips static serving — the API still works.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

// The "all:" prefix includes hidden files (like .gitkeep) so the embed
// directive compiles even when no real frontend build is present.
//
//go:embed all:dist
var embedded embed.FS

// Files returns the embedded frontend as an http.FileSystem, or nil if the
// frontend has not been built (dist/ contains only the placeholder .gitkeep).
// Callers must check for nil before registering the static file handler.
func Files() http.FileSystem {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil
	}
	// Probe for index.html — present in a real build, absent in dev/placeholder.
	f, err := sub.Open("index.html")
	if err != nil {
		return nil
	}
	f.Close()
	return http.FS(sub)
}

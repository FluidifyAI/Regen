# ============================================================================
# Production Dockerfile — builds frontend + backend into a single binary.
# Build context: repo root (.)
#
#   docker build -t fluidify-regen .
#   docker run -p 8080:8080 fluidify-regen
#
# The resulting image serves both the React UI and the API from :8080.
# No CORS configuration is needed — same origin, no cross-origin requests.
# ============================================================================

# ── Stage 1: Build the React frontend ────────────────────────────────────────
FROM node:20-alpine AS frontend-builder

WORKDIR /app

# Install dependencies first (layer-cached when package.json unchanged)
COPY frontend/package*.json ./
RUN npm ci --silent

# Build the SPA
COPY frontend/ ./
RUN npm run build
# Output: /app/dist/

# ── Stage 2: Build the Go binary with embedded frontend ──────────────────────
FROM golang:1.22-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

# Cache Go module downloads separately from source
COPY backend/go.mod backend/go.sum* ./
RUN go mod download

# Copy backend source
COPY backend/ ./

# Embed the frontend: place Vite output where //go:embed all:dist expects it
COPY --from=frontend-builder /app/dist ./ui/dist

# Compile — CGO disabled for a fully static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags '-static'" \
    -a -installsuffix cgo \
    -o /bin/regen \
    ./cmd/regen

# ── Stage 3: Minimal production image ────────────────────────────────────────
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

# Non-root user for security
RUN addgroup -g 1001 -S regen && \
    adduser  -u 1001 -S regen -G regen

WORKDIR /app

COPY --from=backend-builder --chown=regen:regen /bin/regen          /app/regen
COPY --from=backend-builder --chown=regen:regen /app/migrations/    /app/migrations/

USER regen

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/regen", "health"] || exit 1

ENTRYPOINT ["/app/regen"]
CMD ["serve"]

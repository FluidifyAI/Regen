# ============================================================================
# Development Stage - with hot reload using air
# ============================================================================
FROM golang:alpine AS development

# Install air for hot reload (using new module path)
RUN go install github.com/air-verse/air@v1.52.3

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Expose port
EXPOSE 8080

# Run with air for hot reload
CMD ["air", "-c", ".air.toml"]

# ============================================================================
# Builder Stage - compile the binary
# ============================================================================
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
# -ldflags explanation:
#   -w: disable DWARF debugging info (reduces size)
#   -s: disable symbol table (reduces size)
#   -extldflags '-static': create static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags '-static'" \
    -a \
    -installsuffix cgo \
    -o /app/bin/openincident \
    ./cmd/openincident

# Verify binary size (should be < 50MB)
RUN ls -lh /app/bin/openincident && \
    size=$(stat -c%s /app/bin/openincident) && \
    echo "Binary size: $(($size / 1024 / 1024))MB" && \
    [ $size -lt 52428800 ] || (echo "Binary exceeds 50MB limit" && exit 1)

# ============================================================================
# Production Stage - minimal alpine image
# ============================================================================
FROM alpine:3.19 AS production

# Install CA certificates for HTTPS and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1001 -S openincident && \
    adduser -u 1001 -S openincident -G openincident

WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=openincident:openincident /app/bin/openincident /app/openincident

# Copy migrations (will be needed later)
COPY --chown=openincident:openincident migrations/ /app/migrations/

# Switch to non-root user
USER openincident

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/app/openincident", "health"] || exit 1

# Run the binary
ENTRYPOINT ["/app/openincident"]
CMD ["serve"]

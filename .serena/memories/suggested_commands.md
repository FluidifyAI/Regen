# OpenIncident - Suggested Commands

## Development
```bash
make dev          # Start all services (db, redis, api, web)
make backend      # Run backend only (with db and redis)
make frontend     # Run frontend only
make logs         # Show logs from all services
make down         # Stop all services
```

## Database
```bash
make migrate      # Run database migrations
```

## Testing
```bash
make test         # Run full test suite (backend + frontend)
cd backend && go test -v -race -coverprofile=coverage.out ./...  # Backend tests only
cd frontend && npm test  # Frontend tests only
```

## Code Quality
```bash
make fmt          # Format Go and TypeScript code
make lint         # Run linters (Go + TypeScript)
cd backend && go fmt ./...  # Format Go only
cd backend && golangci-lint run  # Lint Go only
```

## Building
```bash
make build        # Build production binaries
make docker       # Build Docker images
make clean        # Remove build artifacts and volumes
```

## Health Check
```bash
make health       # Quick health check of services
```

## Installation
```bash
make install      # Install all dependencies
make install-backend   # Install backend dependencies only
make install-frontend  # Install frontend dependencies only
```

## System Commands (Darwin/macOS)
Standard Unix commands work: `git`, `ls`, `cd`, `grep`, `find`, `cat`, `tail`, `head`, etc.

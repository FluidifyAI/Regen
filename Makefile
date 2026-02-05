.PHONY: help dev backend frontend migrate test fmt lint build clean docker down logs

# Default target
help:
	@echo "OpenIncident Development Commands"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  dev        - Start all services (db, redis, api, web)"
	@echo "  backend    - Run backend only (with db and redis)"
	@echo "  frontend   - Run frontend only"
	@echo "  migrate    - Run database migrations"
	@echo "  test       - Run test suite (backend + frontend)"
	@echo "  fmt        - Format Go and TypeScript code"
	@echo "  lint       - Run linters (Go + TypeScript)"
	@echo "  build      - Build production binaries"
	@echo "  docker     - Build Docker images"
	@echo "  down       - Stop all services"
	@echo "  clean      - Remove build artifacts and volumes"
	@echo "  logs       - Show logs from all services"

# Start all services
dev:
	@echo "Starting all services..."
	docker-compose up -d
	@echo "Services started. Access:"
	@echo "  API: http://localhost:8080"
	@echo "  Web: http://localhost:3000"
	@echo ""
	@echo "View logs: make logs"
	@echo "Stop services: make down"

# Run backend only
backend:
	@echo "Starting backend services (db, redis, api)..."
	docker-compose up -d db redis api
	@echo "Backend started on http://localhost:8080"

# Run frontend only
frontend:
	@echo "Starting frontend service..."
	docker-compose up -d web
	@echo "Frontend started on http://localhost:3000"

# Run database migrations
migrate:
	@echo "Running database migrations..."
	docker-compose exec api go run cmd/openincident/main.go migrate
	@echo "Migrations complete"

# Run test suite
test:
	@echo "Running backend tests..."
	cd backend && go test -v -race -coverprofile=coverage.out ./...
	@echo ""
	@echo "Running frontend tests..."
	cd frontend && npm test
	@echo ""
	@echo "All tests passed!"

# Format code
fmt:
	@echo "Formatting Go code..."
	cd backend && go fmt ./...
	@echo "Formatting TypeScript code..."
	cd frontend && npm run format
	@echo "Code formatted!"

# Run linters
lint:
	@echo "Linting Go code..."
	cd backend && golangci-lint run
	@echo "Linting TypeScript code..."
	cd frontend && npm run lint
	@echo "Linting complete!"

# Build production binaries
build:
	@echo "Building backend binary..."
	cd backend && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/openincident cmd/openincident/main.go
	@echo "Building frontend..."
	cd frontend && npm run build
	@echo "Build complete!"
	@echo "  Backend binary: backend/bin/openincident"
	@echo "  Frontend build: frontend/dist/"

# Build Docker images
docker:
	@echo "Building Docker images..."
	docker-compose build
	@echo "Docker images built!"

# Stop all services
down:
	@echo "Stopping all services..."
	docker-compose down
	@echo "Services stopped"

# Clean build artifacts and volumes
clean:
	@echo "Cleaning build artifacts..."
	rm -rf backend/bin
	rm -rf frontend/dist
	rm -rf backend/coverage.out
	@echo "Removing Docker volumes..."
	docker-compose down -v
	@echo "Clean complete!"

# Show logs from all services
logs:
	docker-compose logs -f

# Quick health check
health:
	@echo "Checking service health..."
	@curl -s http://localhost:8080/health || echo "API not responding"
	@curl -s http://localhost:3000 > /dev/null && echo "Frontend: OK" || echo "Frontend not responding"

# Development helpers
install-backend:
	@echo "Installing backend dependencies..."
	cd backend && go mod download
	@echo "Backend dependencies installed"

install-frontend:
	@echo "Installing frontend dependencies..."
	cd frontend && npm install
	@echo "Frontend dependencies installed"

install: install-backend install-frontend
	@echo "All dependencies installed!"

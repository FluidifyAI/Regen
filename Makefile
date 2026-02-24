.PHONY: help dev dev-local backend backend-deps frontend frontend-local migrate test fmt lint build clean docker down logs teams-app-package helm-deps helm-lint helm-template helm-test

# Default target
help:
	@echo "OpenIncident Development Commands"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Development:"
	@echo "  dev-local    - Start backend (Docker) + frontend (local npm) - RECOMMENDED"
	@echo "  backend-deps - Start just db + redis for local backend dev"
	@echo "  frontend-local - Run frontend dev server (requires backend running)"
	@echo "  dev          - Start all services via Docker Compose"
	@echo ""
	@echo "Services:"
	@echo "  backend      - Run backend only (with db and redis)"
	@echo "  frontend     - Run frontend only (Docker)"
	@echo "  migrate      - Run database migrations"
	@echo ""
	@echo "Code Quality:"
	@echo "  test         - Run test suite (backend + frontend)"
	@echo "  fmt          - Format Go and TypeScript code"
	@echo "  lint         - Run linters (Go + TypeScript)"
	@echo ""
	@echo "Build & Deploy:"
	@echo "  build        - Build production binaries"
	@echo "  docker       - Build Docker images"
	@echo ""
	@echo "Utilities:"
	@echo "  down                - Stop all services"
	@echo "  clean               - Remove build artifacts and volumes"
	@echo "  logs                - Show logs from all services"
	@echo "  health              - Check service health"
	@echo "  teams-app-package   - Generate Teams bot app package for sideloading"

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

# LOCAL DEVELOPMENT (Recommended for frontend work)
dev-local:
	@echo "🚀 Starting local development environment..."
	@echo ""
	@echo "Starting backend services (PostgreSQL + Redis + API)..."
	@docker-compose up -d db redis api
	@echo "⏳ Waiting for backend to be ready..."
	@sleep 3
	@echo ""
	@echo "✅ Backend started on http://localhost:8080"
	@echo ""
	@echo "📦 Installing frontend dependencies (if needed)..."
	@cd frontend && npm install --silent
	@echo ""
	@echo "🎨 Starting frontend dev server on http://localhost:3000"
	@echo ""
	@cd frontend && npm run dev

# Start just database dependencies for local backend development
backend-deps:
	@echo "Starting backend dependencies (db + redis)..."
	@docker-compose up -d db redis
	@echo "✅ PostgreSQL (5432) and Redis (6379) started"
	@echo "💡 Run backend locally: cd backend && go run cmd/openincident/main.go"

# Run frontend dev server locally (requires backend running)
frontend-local:
	@echo "Starting frontend dev server..."
	@cd frontend && npm run dev

# DOCKER DEVELOPMENT
# Run backend only
backend:
	@echo "Starting backend services (db, redis, api)..."
	@docker-compose up -d db redis api
	@echo "Backend started on http://localhost:8080"

# Run frontend only (Docker)
frontend:
	@echo "Starting frontend service..."
	@docker-compose up -d web
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
	@docker-compose down
	@echo "Services stopped"

# Stop just backend
down-backend:
	@echo "Stopping backend services..."
	@docker-compose stop api
	@echo "Backend API stopped (db and redis still running)"

# Stop just dependencies
down-deps:
	@echo "Stopping db and redis..."
	@docker-compose stop db redis
	@echo "Database dependencies stopped"

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

# Generate Teams app package for sideloading
teams-app-package:
	@./scripts/teams-app-package.sh

## Helm
.PHONY: helm-deps helm-lint helm-template helm-test

helm-deps: ## Download Helm chart dependencies
	helm dependency update deploy/helm/openincident

helm-lint: helm-deps ## Lint the Helm chart
	helm lint deploy/helm/openincident

helm-template: helm-deps ## Dry-run render the chart (requires kubectl context)
	helm template openincident deploy/helm/openincident \
		--set ingress.host=localhost \
		| kubectl apply --dry-run=client -f -

helm-test: helm-lint ## Run all Helm checks (lint + template render)
	helm template openincident deploy/helm/openincident \
		--set ingress.host=localhost > /dev/null && \
		echo "✓ helm template: OK"

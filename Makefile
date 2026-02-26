.PHONY: help dev dev-docker backend migrate test fmt lint build docker down clean logs health install teams-app-package helm-deps helm-lint helm-template helm-test

# ── Help ──────────────────────────────────────────────────────────────────────

help:
	@echo "OpenIncident — Make targets"
	@echo ""
	@echo "Development:"
	@echo "  dev          Start backend in Docker (Air hot-reload) + Vite frontend locally"
	@echo "               This is the standard workflow for active development."
	@echo "  dev-docker   Start everything in Docker (production-like, no hot-reload)"
	@echo "  backend      Start db + redis + api in Docker only (no frontend)"
	@echo "  migrate      Run database migrations inside the running api container"
	@echo "  install      Install all backend and frontend dependencies"
	@echo ""
	@echo "Code quality:"
	@echo "  test         Run Go and frontend test suites"
	@echo "  lint         Run golangci-lint and eslint"
	@echo "  fmt          Format Go and TypeScript code"
	@echo ""
	@echo "Build & Deploy:"
	@echo "  build        Build production Go binary + frontend bundle"
	@echo "  docker       Build Docker images via docker-compose"
	@echo "  teams-app-package  Generate Teams bot app zip for sideloading"
	@echo ""
	@echo "Utilities:"
	@echo "  down         Stop and remove all containers"
	@echo "  clean        Remove containers, volumes, and build artifacts"
	@echo "  logs         Tail logs from all running containers"
	@echo "  health       Check API liveness and readiness"
	@echo ""
	@echo "Helm:"
	@echo "  helm-lint    Lint the Helm chart"
	@echo "  helm-template  Dry-run render the chart"
	@echo "  helm-test    Run all Helm checks"

# ── Development ───────────────────────────────────────────────────────────────

# Standard development workflow:
#   - db, redis, and api run in Docker with Air hot-reload (backend changes rebuild instantly)
#   - Frontend runs locally with Vite for fast HMR
#
# Prerequisites: Docker running, Node.js installed locally.
dev:
	@echo "Starting backend services (db + redis + api)..."
	@docker-compose up -d db redis api
	@echo "Waiting for API to be ready..."
	@sleep 3
	@echo "API running at http://localhost:8080"
	@echo ""
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install --silent
	@echo "Starting Vite dev server at http://localhost:3000"
	@echo ""
	@cd frontend && npm run dev

# Full Docker workflow — everything containerised, no local processes.
# Use this to test production-like builds or when you don't have Node.js locally.
dev-docker:
	@echo "Starting all services in Docker..."
	@docker-compose up -d
	@echo ""
	@echo "  API:      http://localhost:8080"
	@echo "  Frontend: http://localhost:3000"
	@echo ""
	@echo "View logs: make logs"
	@echo "Stop:      make down"

# Start infra + API only (no frontend).
# Useful when working on backend with a separately running frontend, or API-only testing.
backend:
	@echo "Starting backend (db + redis + api)..."
	@docker-compose up -d db redis api
	@echo "API running at http://localhost:8080"

# Run migrations inside the running api container.
migrate:
	@echo "Running database migrations..."
	@docker-compose exec api go run cmd/openincident/main.go migrate
	@echo "Migrations complete"

# ── Code Quality ──────────────────────────────────────────────────────────────

test:
	@echo "Running backend tests..."
	@cd backend && go test -race -coverprofile=coverage.out ./...
	@echo ""
	@echo "Running frontend type check..."
	@cd frontend && npx tsc --noEmit
	@echo ""
	@echo "All checks passed"

lint:
	@echo "Linting Go code..."
	@cd backend && golangci-lint run
	@echo "Linting TypeScript..."
	@cd frontend && npm run lint
	@echo "Lint complete"

fmt:
	@echo "Formatting Go code..."
	@cd backend && go fmt ./...
	@echo "Formatting TypeScript..."
	@cd frontend && npm run format
	@echo "Format complete"

# ── Build & Deploy ────────────────────────────────────────────────────────────

build:
	@echo "Building backend binary..."
	@cd backend && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/openincident cmd/openincident/main.go
	@echo "Building frontend bundle..."
	@cd frontend && npm run build
	@echo ""
	@echo "Artifacts:"
	@echo "  backend/bin/openincident"
	@echo "  frontend/dist/"

docker:
	@echo "Building Docker images..."
	@docker-compose build
	@echo "Done"

teams-app-package:
	@./scripts/teams-app-package.sh

# ── Utilities ─────────────────────────────────────────────────────────────────

down:
	@docker-compose down

clean:
	@echo "Removing build artifacts..."
	@rm -rf backend/bin backend/coverage.out frontend/dist
	@echo "Removing containers and volumes..."
	@docker-compose down -v
	@echo "Clean complete"

logs:
	@docker-compose logs -f

health:
	@echo "Checking API health..."
	@curl -sf http://localhost:8080/health && echo "" || echo "API not responding at :8080"
	@echo "Checking API readiness..."
	@curl -sf http://localhost:8080/ready && echo "" || echo "API not ready"

install:
	@echo "Installing backend dependencies..."
	@cd backend && go mod download
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install
	@echo "Done"

# ── Helm ──────────────────────────────────────────────────────────────────────

helm-deps:
	@helm dependency update deploy/helm/openincident

helm-lint: helm-deps
	@helm lint deploy/helm/openincident

helm-template: helm-deps
	@helm template openincident deploy/helm/openincident \
		--set ingress.host=localhost \
		| kubectl apply --dry-run=client -f -

helm-test: helm-lint
	@helm template openincident deploy/helm/openincident \
		--set ingress.host=localhost > /dev/null && \
		echo "helm template: OK"

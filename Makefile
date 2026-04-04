.PHONY: help start stop dev dev-docker backend migrate test fmt lint build build-frontend docker down clean logs health install helm-deps helm-lint helm-template helm-test load-test chaos-db chaos-redis ha-up ha-down

# ── Help ──────────────────────────────────────────────────────────────────────

help:
	@echo "Fluidify Regen — Make targets"
	@echo ""
	@echo "Quick start:"
	@echo "  start        Build and start everything (production mode) — single command"
	@echo "  stop         Stop everything"
	@echo ""
	@echo "Development:"
	@echo "  dev          Start backend in Docker (Air hot-reload) + Vite frontend locally"
	@echo "               Best for active development — full HMR on :3000, API on :8080."
	@echo "  dev-docker   Start everything in Docker (nginx frontend, no local Node needed)"
	@echo "  backend      Start db + redis + api in Docker only (no frontend)"
	@echo "  migrate      Run database migrations inside the running app container"
	@echo "  install      Install all backend and frontend dependencies"
	@echo ""
	@echo "Code quality:"
	@echo "  test         Run Go and frontend test suites"
	@echo "  lint         Run golangci-lint and eslint"
	@echo "  fmt          Format Go and TypeScript code"
	@echo ""
	@echo "Build & Deploy:"
	@echo "  build        Build frontend + copy into backend/ui/dist + compile Go binary"
	@echo "  build-frontend  Build frontend bundle only (frontend/dist + backend/ui/dist)"
	@echo "  docker       Build production Docker image (single binary, embedded frontend)"
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
	@echo ""
	@echo "Reliability:"
	@echo "  load-test    Run all k6 load test scenarios against localhost:8080"
	@echo "  chaos-db     Run DB kill chaos test (docker-compose)"
	@echo "  chaos-redis  Run Redis kill chaos test (docker-compose)"
	@echo "  ha-up        Start the HA stack (Patroni+HAProxy+etcd, Redis Sentinel)"
	@echo "  ha-down      Tear down the HA stack"

# ── Quick Start ───────────────────────────────────────────────────────────────

# Single command to build and run everything in production mode.
# Builds the Docker image (frontend embedded in Go binary) and starts all services.
# Open http://localhost:8080 when ready.
start:
	@echo "Building and starting Fluidify Regen..."
	@docker-compose up --build -d
	@echo ""
	@echo "  App: http://localhost:8080"
	@echo ""
	@echo "View logs: make logs"
	@echo "Stop:      make stop"

stop:
	@docker-compose down

# ── Development ───────────────────────────────────────────────────────────────

# Standard development workflow:
#   - db, redis, and api run in Docker with Air hot-reload (backend changes rebuild instantly)
#   - Frontend runs locally with Vite for fast HMR
#
# Prerequisites: Docker running, Node.js installed locally.
dev:
	@echo "Starting backend services (db + redis + api)..."
	@docker-compose -f docker-compose.dev.yml up -d db redis api
	@echo "Waiting for API to be ready..."
	@sleep 3
	@echo "API running at http://localhost:8080"
	@echo ""
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install --silent
	@echo "Starting Vite dev server at http://localhost:3000"
	@echo ""
	@cd frontend && npm run dev

# Full Docker dev workflow — everything in Docker (nginx frontend + Air backend hot-reload).
# Use this for local testing without Node.js, or to test nginx/proxy configuration.
dev-docker:
	@echo "Starting all services in Docker (db + redis + api + nginx frontend)..."
	@docker-compose -f docker-compose.dev.yml up -d
	@echo ""
	@echo "  Frontend: http://localhost:3000"
	@echo "  API:      http://localhost:8080"
	@echo ""
	@echo "View logs: make logs"
	@echo "Stop:      make down"

# Start infra + API only (no frontend).
# Useful when working on backend with a separately running frontend, or API-only testing.
backend:
	@echo "Starting backend (db + redis + api)..."
	@docker-compose -f docker-compose.dev.yml up -d db redis api
	@echo "API running at http://localhost:8080"

# Run migrations inside the running app container.
migrate:
	@echo "Running database migrations..."
	@docker-compose exec app go run cmd/regen/main.go migrate
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

# build-frontend: compile the React SPA and copy it into backend/ui/dist/ so
# the Go //go:embed picks it up for local binary builds.
build-frontend:
	@echo "Building frontend bundle..."
	@cd frontend && npm run build
	@echo "Copying into backend/ui/dist/ for Go embed..."
	@rm -rf backend/ui/dist && cp -r frontend/dist backend/ui/dist
	@echo "  frontend/dist/  →  backend/ui/dist/"

# build: full local production build — frontend embedded in the Go binary.
# The resulting binary at backend/bin/regen serves UI + API on :8080.
build: build-frontend
	@echo "Compiling Go binary with embedded frontend..."
	@cd backend && CGO_ENABLED=0 GOOS=linux go build \
		-ldflags="-w -s -extldflags '-static'" \
		-o bin/regen \
		./cmd/regen
	@echo ""
	@echo "Artifact: backend/bin/regen (UI + API, no CORS config needed)"

# docker: build the production image using the top-level Dockerfile.
# The image serves both UI and API from :8080 — single binary, zero config.
#
#   docker run -p 8080:8080 -e DATABASE_URL=... -e REDIS_URL=... fluidify-regen
docker:
	@echo "Building production Docker image..."
	@docker build -t fluidify-regen .
	@echo ""
	@echo "Run: docker run -p 8080:8080 fluidify-regen"


# ── Utilities ─────────────────────────────────────────────────────────────────

down:
	@docker-compose down
	@docker-compose -f docker-compose.dev.yml down

clean:
	@echo "Removing build artifacts..."
	@rm -rf backend/bin backend/coverage.out frontend/dist
	@rm -rf backend/ui/dist && mkdir -p backend/ui/dist && touch backend/ui/dist/.gitkeep
	@echo "Removing containers and volumes..."
	@docker-compose down -v
	@docker-compose -f docker-compose.dev.yml down -v
	@echo "Clean complete"

logs:
	@docker-compose logs -f 2>/dev/null || docker-compose -f docker-compose.dev.yml logs -f

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

# ── Reliability ───────────────────────────────────────────────────────────────

load-test:
	@which k6 > /dev/null 2>&1 || (echo "k6 not found. Install: brew install k6" && exit 1)
	@echo "=== Webhook sustained load (5 min, 50 VUs) ==="
	k6 run load-tests/webhook-sustained.js
	@echo ""
	@echo "=== Webhook burst / flood protection ==="
	k6 run load-tests/webhook-burst.js
	@echo ""
	@echo "=== Concurrent API reads ==="
	@echo "Note: set AUTH_TOKEN=<token> for authenticated endpoints"
	k6 run load-tests/api-read.js

chaos-db:
	@bash scripts/chaos/db-kill.sh

chaos-redis:
	@bash scripts/chaos/redis-kill.sh

ha-up:
	@echo "Starting HA stack (PostgreSQL primary+replica+PgBouncer, Redis Sentinel)..."
	docker-compose -f docker-compose.ha.yml up -d
	@echo "Waiting for services to be healthy..."
	@sleep 10
	@echo "HA stack ready."
	@echo "  DB (via PgBouncer):  postgresql://regen:secret@localhost:5433/regen"
	@echo "  Redis Sentinel:      localhost:26379,26380,26381 master=mymaster"
	@echo "  API:                 http://localhost:8080"

ha-down:
	docker-compose -f docker-compose.ha.yml down -v

# ── Helm ──────────────────────────────────────────────────────────────────────

helm-deps:
	@helm dependency update deploy/helm/fluidify-regen

helm-lint: helm-deps
	@helm lint deploy/helm/fluidify-regen

helm-template: helm-deps
	@helm template fluidify-regen deploy/helm/fluidify-regen \
		--set ingress.host=localhost \
		| kubectl apply --dry-run=client -f -

helm-test: helm-lint
	@helm template fluidify-regen deploy/helm/fluidify-regen \
		--set ingress.host=localhost > /dev/null && \
		echo "helm template: OK"

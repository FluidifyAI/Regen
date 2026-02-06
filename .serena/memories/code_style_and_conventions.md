# OpenIncident - Code Style and Conventions

## Go (Backend)
- **Formatting:** `gofmt` (enforced via `make fmt`)
- **Linting:** `golangci-lint` (enforced via `make lint`)
- **Package Structure:** Standard Go layout
  - `cmd/` - Main applications
  - `internal/` - Private application code
  - `internal/models/` - Data models
  - `internal/api/` - API layer (handlers, routes, middleware)
  - `internal/services/` - Business logic
  - `internal/repository/` - Data access layer
  - `internal/worker/` - Background workers
  - `internal/config/` - Configuration management
  - `migrations/` - Database migrations

## TypeScript (Frontend)
- **Formatting:** Prettier (enforced via `npm run format`)
- **Linting:** ESLint (enforced via `npm run lint`)
- **Strict Mode:** Enabled

## Git Commits
- **Convention:** Conventional commits format
  - `feat:` - New features
  - `fix:` - Bug fixes
  - `docs:` - Documentation changes
  - `refactor:` - Code refactoring
  - `test:` - Adding/modifying tests
  - `chore:` - Maintenance tasks

## Key Architectural Principles
1. **Immutable Audit Trail** - All `received_at`, `created_at`, `timestamp` fields are server-generated and immutable
2. **Timeline Entries** - Cannot be updated or deleted (compliance requirement)
3. **Slack-First, Chat-Agnostic** - Build Slack integration first, use `ChatService` interface for abstraction
4. **Integrate, Don't Replace** - Sit alongside existing observability stack via webhooks
5. **AI is Optional** - Product works 100% without AI configured
6. **No Premature Abstraction** - Simple first, refactor when patterns emerge

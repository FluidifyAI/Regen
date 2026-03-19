# Contributing to Fluidify Regen

Thank you for your interest in contributing to Fluidify Regen! We welcome contributions from the community.

This guide will help you get started with development and explain our contribution process.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Code Style Guidelines](#code-style-guidelines)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Commit Message Format](#commit-message-format)
- [Review Process](#review-process)
- [Getting Help](#getting-help)

---

## Code of Conduct

Please be respectful and constructive in all interactions. We are building a welcoming community.

**Key principles**:
- Be respectful of differing opinions
- Accept constructive criticism gracefully
- Focus on what is best for the community
- Show empathy towards other contributors

---

## Getting Started

### Finding Work

**Good first issues**: Check the `good-first-issue` label in GitHub Issues.

**Feature requests**: Check open issues or propose new features in GitHub Discussions.

**Bug reports**: If you find a bug, search existing issues first. If not found, create a new issue with:
- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment details (OS, Go version, Docker version)

---

## Development Setup

### Prerequisites

- **Go**: 1.24 or later ([install guide](https://go.dev/doc/install))
- **Docker & Docker Compose**: For PostgreSQL and Redis
- **Node.js**: 20.x or later (for frontend development)
- **Git**: For version control
- **Make**: For build automation (optional but recommended)

### Initial Setup

1. **Fork the repository**
   ```bash
   # On GitHub, click "Fork" button
   # Then clone your fork:
   git clone https://github.com/YOUR_USERNAME/Regen.git
   cd Regen
   ```

2. **Add upstream remote**
   ```bash
   git remote add upstream https://github.com/FluidifyAI/Regen.git
   ```

3. **Start dependencies**
   ```bash
   # Start PostgreSQL and Redis
   docker-compose up -d db redis

   # Verify they're running
   docker-compose ps
   ```

4. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env if needed (default values work for local development)
   ```

5. **Run backend**
   ```bash
   cd backend
   go mod download
   go run ./cmd/regen
   ```

   Backend runs at: http://localhost:8080

6. **Run frontend (separate terminal)**
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

   Frontend runs at: http://localhost:3000

### Verify Installation

```bash
# Check backend health
curl http://localhost:8080/health

# Check readiness
curl http://localhost:8080/ready

# Send test alert
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "test",
    "status": "firing",
    "alerts": [{
      "status": "firing",
      "labels": {"alertname": "TestAlert", "severity": "critical"},
      "annotations": {"summary": "Test alert"},
      "startsAt": "2024-01-01T00:00:00Z"
    }]
  }'

# Verify incident was created
curl http://localhost:8080/api/v1/incidents
```

### Development Workflow

```bash
# Create a feature branch
git checkout -b feature/your-feature-name

# Make your changes...

# Run tests
make test

# Run linters
make lint

# Format code
make fmt

# Build
make build

# Commit and push
git add .
git commit -m "feat: add your feature"
git push origin feature/your-feature-name
```

---

## Project Structure

```
regen/
├── backend/                  # Go backend
│   ├── cmd/regen/     # Main application entry point
│   ├── internal/
│   │   ├── api/              # API routes and handlers
│   │   │   ├── handlers/     # HTTP request handlers
│   │   │   └── middleware/   # HTTP middleware (logging, CORS, etc.)
│   │   ├── models/           # Database models (GORM)
│   │   ├── repository/       # Data access layer
│   │   ├── services/         # Business logic
│   │   ├── config/           # Configuration loading
│   │   ├── database/         # Database connection and migrations
│   │   ├── redis/            # Redis client
│   │   └── metrics/          # Prometheus metrics
│   ├── migrations/           # SQL migration files
│   └── go.mod
│
├── frontend/                 # React frontend
│   ├── src/
│   │   ├── api/              # API client
│   │   ├── components/       # React components
│   │   ├── pages/            # Page-level components
│   │   └── App.tsx
│   └── package.json
│
├── docs/                     # Documentation
│   ├── CLAUDE.md             # Project context
│   ├── PRODUCT.md            # Product vision
│   ├── ARCHITECTURE.md       # System design
│   ├── DECISIONS.md          # ADRs
│   └── API.md                # API reference
│
├── jira/                     # Epic tracking (JSON)
├── docker-compose.yml        # Local development environment
├── Makefile                  # Build automation
└── README.md
```

---

## Code Style Guidelines

We follow standard Go and TypeScript conventions. See [DECISIONS.md](docs/DECISIONS.md) for architecture decisions.

### Go (Backend)

**Formatting**:
- Use `gofmt` (built into Go)
- Use `goimports` for import organization
- Run `make fmt` before committing

**Linting**:
- We use `golangci-lint` with strict settings
- Run `make lint` to check
- All linter warnings must be fixed before merging

**Naming**:
- Use `camelCase` for local variables
- Use `PascalCase` for exported types and functions
- Use descriptive names (avoid single-letter variables except in loops)

**Error Handling**:
- Always handle errors explicitly
- Use `fmt.Errorf` with `%w` for error wrapping
- Log errors with structured logging (`slog`)

**Example**:
```go
// Good
func CreateIncident(title string, severity string) (*Incident, error) {
    if title == "" {
        return nil, fmt.Errorf("title is required")
    }

    incident := &Incident{
        Title:    title,
        Severity: severity,
        Status:   IncidentStatusTriggered,
    }

    if err := db.Create(incident).Error; err != nil {
        return nil, fmt.Errorf("failed to create incident: %w", err)
    }

    slog.Info("incident created", "id", incident.ID, "title", incident.Title)
    return incident, nil
}
```

**Logging**:
- Use `slog` (structured logging)
- Include relevant context (request ID, incident ID, etc.)
- Use appropriate levels: `Debug`, `Info`, `Warn`, `Error`

**Database**:
- Use GORM for ORM
- Always use transactions for multi-step operations
- Use migrations for schema changes (never modify database directly)

### TypeScript (Frontend)

**Formatting**:
- Use Prettier (run `npm run format`)
- 2-space indentation
- Semicolons required
- Single quotes for strings

**Linting**:
- ESLint with TypeScript rules
- Run `npm run lint` before committing

**Naming**:
- Use `camelCase` for variables and functions
- Use `PascalCase` for components and types
- Use `UPPER_CASE` for constants

**Components**:
- Use functional components with hooks
- Keep components focused (single responsibility)
- Extract reusable logic into custom hooks

### SQL Migrations

**File naming**:
```
YYYYMMDDHHMMSS_description.up.sql
YYYYMMDDHHMMSS_description.down.sql
```

Example:
```
20240115120000_create_incidents_table.up.sql
20240115120000_create_incidents_table.down.sql
```

**Migration guidelines**:
- Always provide both `up` and `down` migrations
- Use `IF NOT EXISTS` for idempotency
- Add indexes for commonly queried fields
- Never modify existing migrations (create new ones instead)

---

## Testing Requirements

### Backend Tests

**Unit tests**:
- Test files: `*_test.go` in same directory as source
- Run: `go test ./...` or `make test`
- Coverage target: >70% for new code

**Integration tests**:
- Located in `internal/api/handlers/*_test.go`
- Test HTTP endpoints with real database (test containers)
- Use `testify` for assertions

**Example**:
```go
func TestCreateIncident(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    // Create test data
    svc := services.NewIncidentService(...)

    // Test
    incident, err := svc.CreateIncident("Test", "high", "")
    assert.NoError(t, err)
    assert.Equal(t, "Test", incident.Title)
    assert.Equal(t, "high", incident.Severity)
}
```

### Frontend Tests

**Component tests**:
- Use Jest + React Testing Library
- Test user interactions, not implementation details
- Run: `npm test`

**Example**:
```typescript
test('renders incident list', async () => {
  render(<IncidentList />);
  expect(await screen.findByText('Incident #1')).toBeInTheDocument();
});
```

### Running Tests

```bash
# Backend tests
cd backend
go test ./...
go test -race ./...          # With race detector
go test -cover ./...         # With coverage

# Frontend tests
cd frontend
npm test
npm test -- --coverage       # With coverage

# All tests
make test
```

---

## Pull Request Process

### Before Submitting

1. **Sync with upstream**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run full test suite**
   ```bash
   make test
   make lint
   make build
   ```

3. **Update documentation** if needed (README, API docs, etc.)

4. **Add tests** for new features

### Submitting the PR

1. **Push to your fork**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create PR on GitHub**
   - Go to https://github.com/FluidifyAI/Regen
   - Click "New Pull Request"
   - Select your fork and branch

3. **Fill out PR template**
   - Describe what changed and why
   - Link related issues (`Fixes #123`)
   - Add screenshots for UI changes
   - List breaking changes (if any)

### PR Title Format

Use conventional commits format:

```
type(scope): description

Examples:
feat(api): add incident timeline endpoint
fix(slack): handle channel creation errors
docs(readme): update setup instructions
refactor(db): optimize incident queries
test(handlers): add integration tests for webhooks
```

**Types**:
- `feat` — New feature
- `fix` — Bug fix
- `docs` — Documentation changes
- `style` — Code style changes (formatting, no logic change)
- `refactor` — Code refactoring (no behavior change)
- `test` — Adding or updating tests
- `chore` — Build/tooling changes

### PR Checklist

- [ ] Tests pass (`make test`)
- [ ] Linters pass (`make lint`)
- [ ] Code formatted (`make fmt`)
- [ ] Documentation updated (if needed)
- [ ] Commit messages follow conventional commits
- [ ] No merge conflicts with `main`
- [ ] PR description is clear and complete

---

## Commit Message Format

We use **Conventional Commits** for clear, semantic commit history.

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Examples**:

```
feat(api): add Prometheus webhook handler

Implements Alertmanager webhook ingestion with:
- Alert deduplication by fingerprint
- Automatic incident creation for critical/warning alerts
- Timeline entry generation

Closes #42
```

```
fix(slack): handle channel name collisions

When channel name already exists, append numeric suffix (-1, -2, etc.)
instead of failing.

Fixes #56
```

```
docs(api): document incident endpoints

Add comprehensive API documentation with:
- Request/response examples
- Error codes
- Full Alertmanager payload example

Related to #48
```

### Types

- `feat` — New feature for the user (not a new feature for build script)
- `fix` — Bug fix for the user (not a fix to a build script)
- `docs` — Documentation changes
- `style` — Formatting, missing semicolons, etc. (no code change)
- `refactor` — Refactoring code (no behavior change)
- `perf` — Performance improvements
- `test` — Adding tests, refactoring tests (no production code change)
- `chore` — Updating build tasks, dependencies, etc. (no production code change)

### Scope (Optional but Recommended)

- `api` — API endpoints/handlers
- `models` — Database models
- `services` — Business logic
- `slack` — Slack integration
- `ui` — Frontend components
- `db` — Database/migrations
- `docs` — Documentation
- `ci` — CI/CD pipelines

### Breaking Changes

If your commit introduces breaking changes, add `BREAKING CHANGE:` in the footer:

```
feat(api)!: change incident response format

BREAKING CHANGE: The /api/v1/incidents endpoint now returns a wrapped
response with `data` and `meta` fields instead of a plain array.

Migration guide:
- Before: GET /incidents → [ {...}, {...} ]
- After:  GET /incidents → { "data": [...], "meta": {...} }
```

---

## Review Process

### What to Expect

1. **Automated checks** run on all PRs (tests, linting, build)
2. **Maintainer review** within 2-3 business days
3. **Feedback cycle** — address comments, push updates
4. **Approval** — requires 1 maintainer approval
5. **Merge** — maintainer merges (squash and merge)

### Review Criteria

- **Functionality**: Does it work as intended?
- **Tests**: Are there sufficient tests?
- **Code quality**: Is it readable and maintainable?
- **Documentation**: Are docs updated?
- **Breaking changes**: Are they necessary and documented?

### Addressing Feedback

```bash
# Make requested changes
git add .
git commit -m "fix: address review comments"
git push origin feature/your-feature-name
```

**Tip**: Respond to each comment on GitHub to track what's been addressed.

---

## Getting Help

### Questions?

- **GitHub Discussions**: For general questions and ideas
- **GitHub Issues**: For bug reports and feature requests
- **Discord**: [Join our community](https://discord.gg/fluidify) (coming soon)

### Stuck on Something?

- Check existing issues for similar problems
- Read the documentation: [docs/](docs/)
- Ask in GitHub Discussions

### Need Guidance?

For large features or architectural changes:
1. **Open an issue first** to discuss the approach
2. **Get feedback** from maintainers before implementing
3. **Break it into smaller PRs** if possible

---

## License

By contributing, you agree that your contributions will be licensed under the **AGPLv3 License** (same as the project).

---

## Recognition

All contributors will be recognized in our README. Thank you for helping make Fluidify Regen better! 🎉

---

**Ready to contribute?**

1. Find an issue or create a new one
2. Fork and clone the repo
3. Create a feature branch
4. Make your changes
5. Submit a pull request

We look forward to your contributions!

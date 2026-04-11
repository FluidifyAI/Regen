# Contributing to Fluidify Regen

Thank you for your interest in contributing! Fluidify Regen is open-source and community-driven — all contributions are welcome.

---

## Table of Contents

- [Finding Work](#finding-work)
- [Development Setup](#development-setup)
- [Branch Naming](#branch-naming)
- [Commit Messages](#commit-messages)
- [Code Style](#code-style)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Getting Help](#getting-help)

---

## Finding Work

- **Good first issues** — [`good-first-issue`](https://github.com/FluidifyAI/Regen/issues?q=is%3Aissue+is%3Aopen+label%3A%22good-first-issue%22) label
- **Feature requests** — open or upvote in [Discussions](https://github.com/FluidifyAI/Regen/discussions)
- **Bug reports** — search [Issues](https://github.com/FluidifyAI/Regen/issues) first; if not found, open one with steps to reproduce, expected vs actual behaviour, and your environment (OS, Go version, Docker version)
- **Large changes** — open a Discussion before implementing so we can align on approach

---

## Development Setup

### Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.24+ |
| Node.js | 20.x+ |
| Docker + Compose | any recent |
| Make | optional but handy |

### First-time setup

```bash
# 1. Fork on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/Regen.git
cd Regen

# 2. Add upstream
git remote add upstream https://github.com/FluidifyAI/Regen.git

# 3. Copy env file
cp .env.example .env   # defaults work for local dev

# 4. Start dependencies
docker-compose up -d db redis

# 5. Run backend
cd backend && go run ./cmd/regen/... serve

# 6. Run frontend (separate terminal)
cd frontend && npm install && npm run dev
```

- Backend: http://localhost:8080
- Frontend: http://localhost:3000

### Verify it works

```bash
curl http://localhost:8080/health

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

curl http://localhost:8080/api/v1/incidents
```

---

## Branch Naming

```
<type>/<short-description>
<type>/<ticket>-<short-description>
```

| Type | When to use |
|------|------------|
| `feat/` | New feature |
| `fix/` | Bug fix |
| `chore/` | Build, tooling, deps |
| `docs/` | Documentation only |
| `refactor/` | Refactor with no behaviour change |
| `test/` | Tests only |

**Examples**
```
feat/prometheus-webhook
fix/ope-42-slack-channel-collision
chore/upgrade-go-1.24
docs/helm-setup-guide
```

Keep branch names lowercase and hyphen-separated. Avoid slashes beyond the type prefix.

---

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/).

```
<type>(<scope>): <subject>

<body — optional, wrap at 72 chars>

<footer — optional, e.g. Closes #42>
```

**Types:** `feat` · `fix` · `docs` · `style` · `refactor` · `perf` · `test` · `chore`

**Scopes:** `api` · `models` · `services` · `slack` · `teams` · `db` · `ui` · `ci` · `helm`

**Examples**

```
feat(api): add Prometheus webhook handler

Implements Alertmanager webhook ingestion with alert deduplication
by fingerprint and automatic incident creation for critical/warning
alerts.

Closes #42
```

```
fix(slack): handle channel name collisions

Append numeric suffix (-1, -2, …) when name already exists instead
of failing with a 409.

Fixes #56
```

**Breaking changes** — add `!` after the type and `BREAKING CHANGE:` in the footer:

```
feat(api)!: wrap list responses in data/meta envelope

BREAKING CHANGE: GET /api/v1/incidents now returns
{ "data": [...], "meta": { "total": N } } instead of a plain array.
```

---

## Code Style

### Go

- Format with `gofmt` / `make fmt`
- Lint with `golangci-lint` / `make lint` — all warnings must be clean before merge
- Handle every error explicitly; wrap with `fmt.Errorf("context: %w", err)`
- Use `slog` for structured logging with relevant fields (incident ID, request ID, etc.)
- Use GORM parameterised queries — no raw string interpolation into SQL
- Use transactions for multi-step DB operations
- Use migrations for all schema changes — never modify the DB directly

### TypeScript

- Format with Prettier / `npm run format`
- Lint with ESLint / `npm run lint`
- Functional components with hooks only — no class components
- 2-space indent, semicolons, single quotes

### SQL Migrations

Files live in `backend/migrations/` and follow a sequential 6-digit prefix:

```
000024_add_foo_table.up.sql
000024_add_foo_table.down.sql
```

Rules:
- Always provide both `up` and `down`
- Use `IF NOT EXISTS` / `IF EXISTS` for idempotency
- Add indexes for columns used in `WHERE` or `JOIN`
- Never modify an existing migration — create a new one

---

## Testing

```bash
# Backend — unit + integration (requires postgres running)
cd backend
go test ./...
go test -race ./...    # race detector
go test -cover ./...   # coverage

# Frontend
cd frontend
npm test
npm test -- --coverage

# Everything
make test
make lint
```

Coverage target for new code: **70%+**

Backend integration tests hit a real database — do not mock the DB layer.

---

## Pull Request Process

### Before opening a PR

```bash
git fetch upstream
git rebase upstream/main

make test
make lint
make build
```

### PR title

Same format as commit messages: `type(scope): description`

### PR description

- What changed and why
- Link related issues (`Closes #123`)
- Screenshots for UI changes
- Call out breaking changes

### Checklist

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make fmt` / `npm run format` run
- [ ] Docs updated if behaviour changed
- [ ] Migration provided if schema changed
- [ ] No merge conflicts with `main`

### Review

- Automated CI runs on every PR (backend, frontend, Docker build)
- One maintainer approval required to merge
- Maintainers merge using **squash and merge**
- Expect feedback within 2–3 business days

---

## Project Structure

```
Regen/
├── CLAUDE.md                     # Project context for AI assistants
├── CONTRIBUTING.md               # This file
├── README.md
├── LICENSE                       # AGPLv3
├── Makefile
├── docker-compose.yml
│
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                # Backend · Frontend · Docker checks
│   │   ├── release.yml           # GHCR image + Helm chart on tag push
│   │   └── k8s-test.yml          # End-to-end Kubernetes tests (self-hosted)
│   └── ISSUE_TEMPLATE/
│
├── backend/
│   ├── cmd/regen/                # main() + CLI commands (serve, migrate)
│   ├── internal/
│   │   ├── api/
│   │   │   ├── routes.go
│   │   │   ├── handlers/         # HTTP request handlers
│   │   │   └── middleware/       # Auth, logging, CORS, rate limiting
│   │   ├── models/               # GORM models
│   │   ├── repository/           # Data access layer
│   │   ├── services/             # Business logic
│   │   ├── config/               # Env-based configuration
│   │   └── worker/               # Background jobs (escalation, shifts)
│   └── migrations/               # Sequential SQL migrations (000001_…)
│
├── frontend/
│   └── src/
│       ├── api/                  # API client
│       ├── components/           # Shared React components
│       └── pages/                # Page-level components
│
├── deploy/
│   ├── helm/fluidify-regen/      # Helm chart
│   ├── grafana/                  # Pre-built dashboard
│   └── kubernetes/               # Raw manifests
│
├── scripts/
│   ├── k8s-test/                 # End-to-end k3d test suite
│   └── chaos/                    # HA chaos scripts
│
└── docs/                         # User-facing documentation
    ├── DECISIONS.md              # Architecture Decision Records
    ├── OPERATIONS.md             # HA, Kubernetes, observability
    └── SECURITY.md               # Security model and hardening
```

---

## Getting Help

- **GitHub Discussions** — questions, ideas, architecture discussion
- **GitHub Issues** — bugs and feature requests
- **Discord** — [discord.gg/b6PSdhzDa](https://discord.gg/b6PSdhzDa)

---

## License

By contributing, you agree your work is licensed under [AGPLv3](LICENSE).

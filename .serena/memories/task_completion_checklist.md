# OpenIncident - Task Completion Checklist

When completing any coding task, follow this checklist:

## 1. Code Quality
```bash
make fmt    # Format all code
make lint   # Run all linters
```

## 2. Testing
```bash
make test   # Run full test suite
```
- Ensure all tests pass
- Add unit tests for new services/handlers
- Add integration tests for new API endpoints
- Add E2E tests for critical flows (alert → incident → Slack)

## 3. Documentation
- Update relevant documentation if behavior changes
- Update ARCHITECTURE.md for architectural changes
- Update DECISIONS.md for new technical decisions
- Keep CLAUDE.md in sync with project state

## 4. Migrations (for database changes)
- Create numbered migration files in `migrations/`
- Provide both up and down migrations
- Test migration with `make migrate`
- Test rollback with `make migrate-down`

## 5. Git Commit
- Use conventional commit format
- Write clear, descriptive commit messages
- Commit in small, testable chunks

## 6. Health Check
```bash
make health  # Verify services are running correctly
```

## Pre-Commit Checklist
- [ ] Code formatted (`make fmt`)
- [ ] Linters pass (`make lint`)
- [ ] Tests pass (`make test`)
- [ ] Migrations tested (if applicable)
- [ ] Documentation updated (if needed)
- [ ] Conventional commit message written

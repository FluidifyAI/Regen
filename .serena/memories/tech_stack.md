# OpenIncident - Tech Stack

| Component | Technology | Why |
|-----------|------------|-----|
| Backend | Go + Gin | Single binary, performance |
| Database | PostgreSQL | JSONB, strong consistency, audit trails |
| Queue/Cache | Redis | Async jobs, caching |
| Frontend | React + TypeScript | Standard, maintainable |
| Chat | Slack (first), Teams (v0.8) | Where incidents happen |
| AI | OpenAI API (BYO key) | No infra to manage |
| Deployment | Docker Compose + Kubernetes | Self-hosted flexibility |

**Current Go Version:** 1.22
**Current Dependencies:** Gin web framework (v1.10.0), godotenv (v1.5.1)

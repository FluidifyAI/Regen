# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest (main) | ✅ |
| older releases | ❌ — please upgrade |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Email **security@fluidify.ai** with the following:

- Description of the vulnerability
- Steps to reproduce
- Potential impact (data exposure, privilege escalation, etc.)
- Any proof-of-concept if available

We will acknowledge within **48 hours** and aim to release a patch within **14 days** for critical issues. We will credit reporters in release notes unless you prefer to remain anonymous.

For lower-severity issues (hardening suggestions, non-exploitable misconfigurations), public issues are welcome.

## Scope

In-scope:
- Backend API (`/api/v1/...`)
- Webhook endpoints (`/api/v1/webhooks/...`)
- Authentication flows (SAML, session management)
- Slack/Teams/Telegram integration security
- Docker Compose and Kubernetes Helm chart defaults

Out of scope:
- Vulnerabilities in third-party dependencies (report to those projects directly)
- Social engineering
- Issues in forks or modified versions

## Security Design Notes

- Webhook payloads are signature-verified (Prometheus/Slack signing secret)
- All `received_at` / `created_at` timestamps are server-generated and immutable
- Session tokens are not logged
- AI API keys (OpenAI) are stored encrypted at rest and never returned in API responses
- CORS is enforced via `CORS_ALLOWED_ORIGINS` allowlist
- Rate limiting is applied at three tiers (global, per-IP, per-endpoint)

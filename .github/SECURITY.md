# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest (main) | ✅ |
| older releases | ❌ — please upgrade |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Email **contact@fluidify.ai** with the following:

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

## Security Architecture

For the full security architecture — including authentication, rate limiting, webhook verification, CSP headers, container hardening, and the production security checklist — see:

**[SECURITY.md](../SECURITY.md)**

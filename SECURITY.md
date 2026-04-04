# Security

## Summary

Fluidify Regen's application layer has no critical vulnerabilities. Authentication is bcrypt-hashed with timing-safe comparison and account lockout. All database queries use GORM parameterized statements. Webhook payloads from Slack and Teams are cryptographically verified. Rate limiting is enforced via a Redis Lua script at three tiers. HTTP responses include a strict Content Security Policy, HSTS, and anti-clickjacking headers. The frontend stores no secrets and uses HTTP-only session cookies.

The items in the production checklist below are infrastructure defaults that must be changed before a public deployment. They are acceptable for open-source defaults but not for production.

---

## Table of Contents

1. [Security Architecture](#1-security-architecture)
2. [Authentication and Session Management](#2-authentication-and-session-management)
3. [Transport and HTTP Security](#3-transport-and-http-security)
4. [Input Handling](#4-input-handling)
5. [Rate Limiting](#5-rate-limiting)
6. [Webhook Authentication](#6-webhook-authentication)
7. [CORS Policy](#7-cors-policy)
8. [Frontend Security](#8-frontend-security)
9. [Infrastructure and Container Security](#9-infrastructure-and-container-security)
10. [CI/CD Security](#10-cicd-security)
11. [Production Security Checklist](#11-production-security-checklist)
12. [Known Limitations and Design Choices](#12-known-limitations-and-design-choices)
13. [Vulnerability Reporting](#13-vulnerability-reporting)

---

## 1. Security Architecture

Security is applied in layers:

| Layer | Mechanism |
|---|---|
| Network | TLS at ingress (operator-managed); HSTS enforced by app |
| Transport | CORS allowlist, security headers on every response |
| Authentication | bcrypt (cost 12), timing-safe comparison, account lockout, HTTP-only SameSite=Strict cookies |
| Authorization | Role check on every protected route (`admin` / `member` / `viewer`) |
| Input | GORM parameterized queries (no raw SQL interpolation), body size cap, panic recovery |
| Rate limiting | Redis Lua script, three tiers (see §5) |
| Webhooks | HMAC-SHA256 or RSA signature verification per source (see §6) |
| Secrets | Environment variables only; never logged, never returned in responses |

---

## 2. Authentication and Session Management

### Local Authentication

- **Password hashing**: bcrypt with cost factor 12 (well above the recommended minimum of 10).
- **Timing-safe comparison**: `bcrypt.CompareHashAndPassword` runs in constant time regardless of whether the password matches, preventing timing attacks.
- **Account lockout**: After 5 consecutive failed logins, the account is locked for 15 minutes. The lockout is tracked server-side in the database — not in the session — so it survives restarts and cannot be bypassed by clearing cookies.
- **Session cookies**: HTTP-only, `SameSite=Strict`, scoped to the server path. The session token is never accessible from JavaScript.
- **Session lifetime**: 24 hours. Expired sessions are rejected at read time (`expires_at > NOW()`); a daily background job purges stale rows.
- **Password reset tokens**: Single-use, short-lived setup tokens issued on account creation and password reset. Tokens are hashed before storage; plaintext is only returned once.

### SAML SSO

- **Standard**: SAML 2.0 Service Provider using the `crewjam/saml` library.
- **JIT provisioning**: Users are created on first SSO login if they don't already exist.
- **User limit enforcement**: New SAML-provisioned users are blocked if the OSS seat limit is reached. Existing users (matched by email or SAML subject) can always log in.
- **Compatible IdPs**: Okta, Azure AD / Entra ID, Google Workspace.
- **Disabled by default**: SSO is a no-op unless `SAML_IDP_METADATA_URL` is set.

### AI Agent Accounts

System agent accounts (`auth_source = 'ai'`) cannot be modified, deactivated, or have their passwords reset through the settings API — protected at the handler level.

---

## 3. Transport and HTTP Security

### Security Headers

Applied to every response by `internal/api/middleware/security.go`:

| Header | Value | Purpose |
|---|---|---|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'` | Restricts resource origins; blocks framing |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` (2 years) | Enforces HTTPS on all future visits |
| `X-Frame-Options` | `DENY` | Secondary clickjacking protection for older browsers |
| `X-Content-Type-Options` | `nosniff` | Prevents MIME sniffing |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limits referrer leakage |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | Disables unused browser APIs |

### Why `X-XSS-Protection` is intentionally omitted

This header is deprecated and has been removed from all modern browsers. In IE and older Edge it can actually *introduce* XSS vulnerabilities by triggering the browser's broken XSS auditor. The backend explicitly omits it with a comment explaining this decision. The nginx development proxy also does not set it.

### HSTS on HTTP

The HSTS header is sent even over HTTP (before TLS termination). This is harmless — browsers only apply HSTS after seeing it over HTTPS — and simplifies the configuration by not requiring conditional logic per-scheme.

---

## 4. Input Handling

- **SQL injection**: All database access uses GORM with parameterized queries. There is no raw SQL string interpolation anywhere in the codebase.
- **Request body limits**: The Gin router caps request bodies at 32 MB by default. Webhook handlers additionally validate payload structure before processing.
- **Panic recovery**: The `gin.Recovery()` middleware catches any unhandled panics and returns a 500 without exposing stack traces to the client.
- **UUID validation**: All path parameters that are UUIDs are parsed with `uuid.Parse()` — invalid values return 400 before hitting any service logic.
- **Duplicate email detection**: User creation checks for PostgreSQL unique constraint violations and returns a structured `duplicate_email` error code rather than leaking database error messages.

---

## 5. Rate Limiting

Rate limiting is implemented as a Redis Lua script (`internal/api/middleware/rate_limit.go`) — atomically increment-and-check in a single round-trip, preventing race conditions.

| Tier | Limit | Window | Applied to |
|---|---|---|---|
| Unauthenticated API | 120 requests | 1 minute | All unauthed API routes |
| Authenticated API | 600 requests | 1 minute | All authed API routes |
| Authentication endpoints | 10 requests | 1 minute | `/api/v1/auth/login`, `/api/v1/auth/bootstrap` |
| Webhooks | 300 requests | 1 minute | All `/api/v1/webhooks/...` routes |

**Fail-open design**: If Redis is unavailable, rate limiting is skipped and requests are allowed through. This is an intentional availability-over-security trade-off: Regen is an incident management platform, and blocking incident ingestion during a Redis outage (which may itself be the incident) is worse than temporarily removing the rate limit. The application logs a warning when fail-open is triggered.

---

## 6. Webhook Authentication

| Source | Method | Replay Protection |
|---|---|---|
| Slack | HMAC-SHA256 over `v0:timestamp:body` using `SLACK_SIGNING_SECRET` | Yes — rejects requests where `X-Slack-Request-Timestamp` is >5 minutes old |
| Microsoft Teams | RSA signature via Bot Framework OIDC (public keys fetched from `https://login.botframework.com/v1/.well-known/openidconfiguration`) | Yes — JWT `exp` claim |
| AWS CloudWatch | RSA signature on SNS message; certificate URL validated against `amazonaws.com` and `amazonaws.com.cn` to prevent SSRF | No — SNS does not include a timestamp in the signature payload |
| Prometheus Alertmanager | URL secrecy only (no built-in signing) | No |
| Grafana | URL secrecy only (no built-in signing) | No |
| Generic | Optional HMAC-SHA256 via `WEBHOOK_SECRET` env var | No |

Prometheus and Grafana do not support request signing in their standard webhook configurations. Protection relies on keeping the webhook URL secret (use a random path segment) and network-level controls (firewall, VPN). This limitation is stated honestly here rather than being obscured.

---

## 7. CORS Policy

CORS is handled by `internal/api/middleware/cors.go`:

- **Allowlist mode**: Only origins listed in `CORS_ALLOWED_ORIGINS` (comma-separated) are permitted.
- **Development fallback**: If `CORS_ALLOWED_ORIGINS` is not set and `APP_ENV != production`, `http://localhost:3000` is automatically allowed to support local development.
- **Production enforcement**: If `APP_ENV=production` and `CORS_ALLOWED_ORIGINS` is empty, no cross-origin requests are allowed (the `Access-Control-Allow-Origin` header is simply not set).
- **Credentials**: `Access-Control-Allow-Credentials: true` is set only when the origin matches the allowlist.

---

## 8. Frontend Security

- **No `dangerouslySetInnerHTML`**: The React codebase does not use this API. All dynamic content is rendered through React's escaped text nodes.
- **HTTP-only cookies**: Session tokens are stored in HTTP-only cookies set by the backend. JavaScript has no access to session credentials.
- **No secrets in the bundle**: The frontend bundle contains no API keys, tokens, or secrets. All sensitive configuration lives in backend environment variables.
- **Content Security Policy**: The CSP set by the backend (see §3) restricts what scripts and resources the frontend can load.
- **SPA serving**: In production the frontend is embedded into the Go binary (`embed.FS`). The backend reads `index.html` at startup and serves it for all non-asset routes; real static assets (JS/CSS) are served by Go's `http.FileServer`.

---

## 9. Infrastructure and Container Security

### Container

- **Non-root user**: The Docker image runs as UID 1001 (`fluidify`), not root.
- **Minimal base image**: `alpine:3.19` — no package manager, shell, or unnecessary tools in the final image.
- **Multi-stage build**: The builder stage uses the full Go toolchain; the final stage copies only the compiled binary.

### Kubernetes

The Helm chart (`deploy/helm/fluidify-regen/`) configures:

```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1001
  fsGroup: 1001

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

- `readOnlyRootFilesystem: true` — the container filesystem is read-only; no writes to the container layer.
- `drop: [ALL]` — all Linux capabilities are dropped. The application requires none.
- SAML SP key/cert must be mounted via a Kubernetes Secret volume — the application reads them from a path, not writing them itself.

---

## 10. CI/CD Security

- **Pinned actions**: All GitHub Actions use pinned SHA hashes (e.g. `actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683`), not floating tags.
- **Minimal token scope**: Workflows use the default `GITHUB_TOKEN` with only the permissions required per job.
- **No fork secret exposure**: `pull_request` triggers do not have access to repository secrets. Secrets are only available in `push` and `workflow_dispatch` contexts.
- **No external action execution on PRs from forks**: Build and test workflows run with read-only token access on fork PRs.

---

## 11. Production Security Checklist

Before deploying Regen to production, complete every **REQUIRED** item.

### REQUIRED

| Item | How |
|---|---|
| TLS at ingress | Configure your ingress controller or reverse proxy to terminate TLS |
| `ingress.tls: true` in Helm values | Set `tls: true` and `tlsSecretName` in `values.yaml`; provision cert via cert-manager |
| `APP_ENV=production` | Ensures production-mode Gin, correct CORS enforcement, and HSTS |
| `CORS_ALLOWED_ORIGINS` set | Comma-separated list of your production origins (e.g. `https://incidents.myco.com`) |
| PostgreSQL password changed | Set `postgresql.auth.password` in Helm values to a strong random value — never deploy `changeme` |
| `DATABASE_URL` with `sslmode=require` | Required if using an external managed database (RDS, Cloud SQL, etc.) |
| Redis auth enabled | Set `redis.auth.enabled: true` and `redis.auth.password` in Helm values |
| Patroni passwords (HA only) | Set `PATRONI_SUPERUSER_PASSWORD`, `PATRONI_REPLICATION_PASSWORD`, `PATRONI_REWIND_PASSWORD` env vars — do not use the defaults |

### RECOMMENDED

| Item | How |
|---|---|
| SAML SSO | Set `SAML_IDP_METADATA_URL` and `SAML_BASE_URL`; configure your IdP |
| `WEBHOOK_SECRET` for generic webhooks | Set to a random 32-byte hex string; share with your webhook sender |
| Kubernetes NetworkPolicy | Restrict pod-to-pod traffic; only allow ingress → app and app → db/redis |
| External managed DB and Redis | Use RDS/Cloud SQL and ElastiCache/Memorystore instead of in-cluster StatefulSets for production data durability |
| SAML SP key rotation schedule | Rotate the SP signing key annually; mount as a Kubernetes Secret |
| Log shipping | Forward structured JSON logs to your SIEM; logs include `user_id`, `ip`, `method`, `path`, `status` |

---

## 12. Known Limitations and Design Choices

**Fail-open rate limiter**: When Redis is unavailable, rate limiting is skipped. This is intentional — an incident management platform must be able to receive alerts even if its rate-limiting backend is down.

**No email delivery**: Regen does not send email. Setup tokens and password reset links are delivered out-of-band (the API returns the token; the admin shares it with the user). This eliminates an entire class of email-based attack surface and avoids requiring SMTP configuration.

**Prometheus/Grafana webhook URL secrecy**: Neither source supports request signing in their standard configurations. The webhook URL acts as a shared secret. Use a randomly generated path segment and restrict network access to the webhook endpoint.

**HSTS on HTTP**: The `Strict-Transport-Security` header is sent on all responses including HTTP. Browsers only honour HSTS received over HTTPS, so this has no practical effect on plain HTTP clients but simplifies configuration.

**CloudWatch SNS replay**: SNS does not include a timestamp in the signature payload, so replay protection is not possible at the signature level. Duplicate alert deduplication (`external_id` uniqueness) provides a functional safeguard against repeated deliveries.

---

## 13. Vulnerability Reporting

**Do not open a public GitHub issue for security vulnerabilities.**

Email **contact@fluidify.ai** with:

- Description of the vulnerability
- Steps to reproduce
- Potential impact (data exposure, privilege escalation, etc.)
- Proof-of-concept if available

We acknowledge within **48 hours** and aim to patch critical issues within **14 days**. Reporters are credited in release notes unless they prefer to remain anonymous.

For non-exploitable misconfigurations or hardening suggestions, public GitHub issues are welcome.

See also: [`.github/SECURITY.md`](.github/SECURITY.md)

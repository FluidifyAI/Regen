# Security Hardening Guide

This document describes OpenIncident's security posture and the configuration steps required to run it securely in production.

---

## HTTP Security Headers

All responses include the following security headers (set by `SecurityHeaders()` middleware):

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevents MIME-type sniffing |
| `X-Frame-Options` | `DENY` | Blocks clickjacking via iframes |
| `X-XSS-Protection` | `1; mode=block` | Legacy XSS filter for older browsers |
| `Content-Security-Policy` | `default-src 'self'` | Blocks inline scripts and external resources |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limits referrer leakage |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` | Forces HTTPS for 2 years |
| `X-Permitted-Cross-Domain-Policies` | `none` | Blocks Flash/PDF cross-domain policy files |

No configuration is needed — these headers are applied automatically.

---

## CORS (Cross-Origin Resource Sharing)

### Why this matters

OpenIncident's API uses session cookies and `Authorization` headers. Reflecting any `Origin` back with `Access-Control-Allow-Credentials: true` is a **critical vulnerability** — it allows any website to make authenticated requests to your API on behalf of your users.

### How it works

CORS is enforced via an **explicit origin allowlist**. Only origins listed in `CORS_ALLOWED_ORIGINS` receive CORS response headers:

- Allowlisted origins: receive `Access-Control-Allow-Origin`, `Access-Control-Allow-Credentials: true`, `Vary: Origin`
- Unknown origins: receive no CORS headers — browser blocks the response (request is still served; browser enforces policy)
- Unknown-origin preflight (`OPTIONS`): returns `403 Forbidden` immediately

### Configuration

Set the `CORS_ALLOWED_ORIGINS` environment variable to a comma-separated list of allowed origins:

```env
# Single origin
CORS_ALLOWED_ORIGINS=https://incidents.myco.com

# Multiple origins
CORS_ALLOWED_ORIGINS=https://incidents.myco.com,https://staging.myco.com
```

**Default (development only):** `http://localhost:3000`

> **Production requirement:** Always set `CORS_ALLOWED_ORIGINS` explicitly. The default `localhost:3000` is intentionally narrow — it will block your production frontend if left unset.

### Docker Compose

```env
# .env
CORS_ALLOWED_ORIGINS=https://incidents.myco.com
```

### Kubernetes (Helm)

```yaml
# values-prod.yaml
config:
  corsAllowedOrigins: "https://incidents.myco.com"
```

---

## Authentication

### SAML SSO (recommended for production)

OpenIncident ships with SAML 2.0 SP support as a free OSS feature. Configure it via environment variables:

```env
SAML_IDP_METADATA_URL=https://your-idp/metadata
SAML_BASE_URL=https://incidents.myco.com
SAML_ENTITY_ID=https://incidents.myco.com/saml/metadata
SAML_CERT_FILE=/run/secrets/saml.crt
SAML_KEY_FILE=/run/secrets/saml.key
```

See the SAML setup guide in `docs/SAML.md` for provider-specific instructions (Okta, Azure AD, Google Workspace).

### Open mode (development only)

When `SAML_IDP_METADATA_URL` is unset, the API runs in **open mode** with no authentication. This is intentional for local development but **must not be used in production**.

> **Production requirement:** Always configure SAML (or a future auth provider) before exposing the API to a network.

---

## Rate Limiting

The API applies three rate-limiting tiers (backed by Redis):

| Tier | Middleware | Limit | Applied to |
|------|-----------|-------|-----------|
| Webhooks | `RateLimitWebhooks()` | 300 req/min per IP | `/api/v1/webhooks/*` |
| API | `RateLimitAPI()` | 120 req/min per IP | All authenticated API routes |
| Auth | `RateLimitAuth()` | 10 req/min per IP | SAML endpoints |

Rate limiting uses a fixed-window counter with an atomic Redis `INCR`+`EXPIRE` script. When Redis is unavailable the API **fails open** (allows all requests) — this is intentional to prevent Redis downtime from blocking incident response.

Standard response headers are set on every rate-limited request:
- `X-RateLimit-Limit` — window limit
- `X-RateLimit-Remaining` — requests remaining
- `X-RateLimit-Reset` — Unix timestamp when the window resets

---

## Webhook Security

Incoming Slack events are verified using Slack's HMAC-SHA256 request signature scheme:

- `SLACK_SIGNING_SECRET` must be set in production
- Requests without a valid signature are rejected with `403`
- Requests with timestamps older than 5 minutes are rejected (replay attack prevention)
- When `SLACK_SIGNING_SECRET` is unset, verification is skipped (dev mode) — **do not leave unset in production**

Prometheus Alertmanager webhooks can be secured with a shared secret via `WEBHOOK_SECRET` (optional). When set, the `X-Webhook-Secret` header is validated on all incoming webhook requests.

---

## Transport Security

### TLS termination

TLS should be terminated at the ingress/load balancer layer. The API itself serves plain HTTP internally.

For Kubernetes deployments, enable TLS in the Helm chart:

```yaml
ingress:
  tls: true
  tlsSecretName: openincident-tls  # managed by cert-manager
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
```

### HSTS

The `Strict-Transport-Security` header is sent on all responses with a 2-year max-age. Browsers that receive this header will refuse to connect over plain HTTP for the duration. This is safe to set at the application layer — HTTP clients ignore it.

---

## Container Security

The Docker image and Helm chart apply these hardening defaults:

- Runs as non-root user (`UID 1001`)
- `readOnlyRootFilesystem: true` — only `/tmp` is writable (via `emptyDir`)
- No new privileges (`allowPrivilegeEscalation: false`)
- No capabilities dropped/added (runs with default minimal set)

---

## Secret Management

### What to keep secret

| Secret | Environment variable | Notes |
|--------|---------------------|-------|
| Database password | `DATABASE_URL` | Embedded in connection string |
| Redis password | `REDIS_URL` | Embedded in connection string |
| Slack bot token | `SLACK_BOT_TOKEN` | `xoxb-` prefix |
| Slack signing secret | `SLACK_SIGNING_SECRET` | Webhook verification |
| SAML SP private key | `SAML_KEY_FILE` | Path to key file |
| OpenAI API key | `OPENAI_API_KEY` | BYO — never logged |
| Teams credentials | `TEAMS_APP_PASSWORD` | Azure app secret |

### Recommendations

**Docker Compose:** Use a `.env` file that is `.gitignore`d. Never commit secrets to git.

**Kubernetes:** Do not use `helm install --set secrets.xxx=...` (plain text in Helm history). Instead:

1. **Sealed Secrets** — encrypt in git, decrypt in-cluster
2. **External Secrets Operator** — sync from AWS Secrets Manager / GCP Secret Manager / Vault
3. **Vault Agent Injector** — inject as env vars at pod start

See `docs/OPERATIONS.md` for detailed examples.

---

## Security Checklist for Production

Before going live, verify:

- [ ] `CORS_ALLOWED_ORIGINS` is set to your production frontend URL(s)
- [ ] `SAML_IDP_METADATA_URL` is configured (not running in open mode)
- [ ] `SLACK_SIGNING_SECRET` is set
- [ ] TLS is terminated at ingress with a valid certificate
- [ ] `POSTGRES_PASSWORD` / `DATABASE_URL` uses a strong, unique password (not `secret` or `changeme`)
- [ ] Redis is not exposed publicly
- [ ] PostgreSQL is not exposed publicly
- [ ] The API is behind an ingress/load balancer (not exposed directly)
- [ ] Secrets are not stored in git or Helm release history
- [ ] `APP_ENV=production` is set (enables structured JSON logging)

---

## Reporting Security Issues

Please report security vulnerabilities privately via GitHub's [security advisory](https://github.com/FluidifyAI/openincident/security/advisories/new) feature rather than opening a public issue. We aim to respond within 48 hours.

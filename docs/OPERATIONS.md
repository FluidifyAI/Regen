# Production Readiness Checklist

Run this checklist on every fresh deployment before going live. Each item links to the relevant documentation section.

**Target environment:** Docker Compose (single-node) or Kubernetes (HA).  
**Time to complete:** ~2 hours for a first-time deployment.

---

## 1. Installation

- [ ] **1.1** Repository cloned from `https://github.com/FluidifyAI/Regen.git`
- [ ] **1.2** `.env` created from `.env.example` — no placeholder values remain (`changeme`, `your-key-here`, etc.)
- [ ] **1.3** `DATABASE_URL` points to a dedicated PostgreSQL 15+ instance (not shared with other services)
- [ ] **1.4** `REDIS_URL` points to a dedicated Redis 7+ instance
- [ ] **1.5** `make start` completes without errors; all three containers (`regen`, `db`, `redis`) are running

---

## 2. Health checks

- [ ] **2.1** `GET /health` returns `{"status":"ok"}` — binary is up
- [ ] **2.2** `GET /ready` returns `{"status":"ready","database":"ok","redis":"ok"}` — dependencies connected
- [ ] **2.3** Uptime monitor configured on `/ready` (not `/health`) — monitors connectivity, not just process liveness

---

## 3. Database

- [ ] **3.1** Migrations ran successfully on first start (check logs: `running database migrations... done`)
- [ ] **3.2** PostgreSQL running with persistent volume — `make stop` + `make start` retains all data
- [ ] **3.3** Daily backup configured — see [backup instructions](#backups)
- [ ] **3.4** Backup restore tested on a separate instance — don't assume backups work until you've restored one
- [ ] **3.5** `DB_MAX_OPEN_CONNS` and `DB_MAX_IDLE_CONNS` set appropriately for expected load (defaults: 25/10)

---

## 4. Security

- [ ] **4.1** `SECRET_KEY` set to a random 32+ character string — used for session signing (`openssl rand -hex 32`)
- [ ] **4.2** `CORS_ALLOWED_ORIGINS` set to your exact frontend domain — wildcard `*` is not acceptable in production
- [ ] **4.3** TLS termination configured — either via reverse proxy (nginx/Caddy/Traefik) or load balancer; HTTP only acceptable in private networks
- [ ] **4.4** `PORT` not exposed directly to the internet — sit behind a reverse proxy
- [ ] **4.5** PostgreSQL and Redis ports (`5432`, `6379`) not exposed outside the host/cluster network
- [ ] **4.6** `.env` file permissions restricted: `chmod 600 .env`
- [ ] **4.7** `APP_ENV=production` set — disables debug output and stack traces in API responses

---

## 5. Rate limiting

- [ ] **5.1** Rate limiting is active (verify: send 100 rapid requests to `/api/v1/incidents` — expect `429` after limit)
- [ ] **5.2** Webhook endpoints have separate rate limit tier — confirm `/api/v1/webhooks/*` accepts higher burst than API endpoints
- [ ] **5.3** Rate limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`) present in API responses

---

## 6. Webhook signing

- [ ] **6.1** `WEBHOOK_SIGNING_SECRET` set if using the generic webhook endpoint — unsigned requests rejected
- [ ] **6.2** Prometheus Alertmanager configured with matching signing secret if webhook signing enabled
- [ ] **6.3** Test webhook delivery end-to-end: fire a test alert → verify incident created in UI

---

## 7. Slack integration

- [ ] **7.1** `SLACK_BOT_TOKEN` set (`xoxb-...`)
- [ ] **7.2** `SLACK_SIGNING_SECRET` set — Slack event verification active
- [ ] **7.3** Slack app installed in target workspace with required scopes (`channels:manage`, `chat:write`, `channels:read`)
- [ ] **7.4** Test: create incident from UI → Slack channel auto-created → status update posts to channel
- [ ] **7.5** Test: type `/incident new` in Slack → incident created in UI

---

## 8. Teams integration (if applicable)

- [ ] **8.1** `TEAMS_APP_ID`, `TEAMS_APP_PASSWORD`, `TEAMS_TENANT_ID`, `TEAMS_TEAM_ID` all set
- [ ] **8.2** `TEAMS_SERVICE_URL` set to correct region (`smba.trafficmanager.net/amer` for US, `/emea` for Europe, `/in` for India)
- [ ] **8.3** Bot sideloaded into the target Team
- [ ] **8.4** Test: create incident → Teams channel created → Adaptive Card posted
- [ ] **8.5** Test: `@Bot ack` in Teams channel → incident acknowledged in UI

---

## 9. AI (if configured)

- [ ] **9.1** `OPENAI_API_KEY` set to a valid key with sufficient quota
- [ ] **9.2** `OPENAI_MODEL` set (default: `gpt-4o-mini`) — verify model is available on your API key tier
- [ ] **9.3** Test: resolve an incident > 5 minutes old → post-mortem draft auto-generated within 60 seconds
- [ ] **9.4** Test: `@bot` mention in Slack channel with a question → response returned within 10 seconds

---

## 10. SSO / SAML (if configured)

- [ ] **10.1** `SAML_IDP_METADATA_URL` set to your IdP metadata URL
- [ ] **10.2** `SAML_BASE_URL` set to the public-facing URL of this Regen instance
- [ ] **10.3** SP metadata (`/auth/saml/metadata`) accessible and registered with IdP
- [ ] **10.4** Test login: redirect to IdP → authenticate → return to Regen → user created/linked
- [ ] **10.5** Test that local login (`/login`) still works for break-glass admin access if SAML is misconfigured

---

## 11. Pro licence (if applicable)

- [ ] **11.1** `REGEN_LICENCE_KEY` set in environment
- [ ] **11.2** Startup logs show: `licence: Pro activated  org=<YourOrg>  seats=<N>`
- [ ] **11.3** Active user count below seat limit (visible in startup log warning if exceeded)
- [ ] **11.4** Licence expiry date noted — set a calendar reminder 30 days before expiry

---

## 12. On-call & escalations

- [ ] **12.1** At least one schedule created with at least one rotation layer and participant
- [ ] **12.2** `GET /api/v1/schedules/:id/oncall` returns current on-call person
- [ ] **12.3** Escalation policy created and linked to an alert routing rule
- [ ] **12.4** Test escalation: trigger alert → incident created → on-call notified via Slack/Teams

---

## 13. Alerting sources

- [ ] **13.1** At least one alert source configured (Prometheus, Grafana, CloudWatch, or generic webhook)
- [ ] **13.2** Test alert received end-to-end: source fires → alert stored → incident auto-created (for critical/warning)
- [ ] **13.3** Alert deduplication working: send same alert twice → only one incident created
- [ ] **13.4** Resolved alert fires → incident status updated (if auto-resolve configured)

---

## 14. Observability

- [ ] **14.1** `GET /metrics` returns Prometheus metrics (if metrics endpoint enabled)
- [ ] **14.2** Application logs structured as JSON (`APP_ENV=production` enables this)
- [ ] **14.3** Log aggregation configured (Loki, CloudWatch, Datadog, etc.) — logs not lost on container restart
- [ ] **14.4** Alert on `/ready` returning non-200 for > 1 minute

---

## 15. Kubernetes / HA (if applicable)

- [ ] **15.1** Helm chart deployed: `helm install regen deploy/helm/fluidify-regen/`
- [ ] **15.2** Minimum 2 replicas configured for the app deployment
- [ ] **15.3** HPA configured (CPU 70% threshold, min 2 / max 10 replicas)
- [ ] **15.4** PodDisruptionBudget set — at least 1 replica always available during node drain
- [ ] **15.5** PostgreSQL HA configured (Patroni, RDS Multi-AZ, CloudNativePG, etc.) — single-node Postgres is a SPOF
- [ ] **15.6** Redis Sentinel or Redis Cluster configured — single-node Redis is a SPOF
- [ ] **15.7** Liveness and readiness probes active on `/health` and `/ready` respectively
- [ ] **15.8** Rolling update strategy configured — zero-downtime deploys verified

---

## Backups

### PostgreSQL backup (Docker Compose)

```bash
# Create backup
docker exec fluidify-regen-db pg_dump -U regen regen | gzip > regen-$(date +%Y%m%d).sql.gz

# Restore backup
gunzip -c regen-20260101.sql.gz | docker exec -i fluidify-regen-db psql -U regen regen
```

### Automate with cron

```bash
# /etc/cron.d/regen-backup — runs daily at 2am
0 2 * * * root docker exec fluidify-regen-db pg_dump -U regen regen | gzip > /backups/regen-$(date +\%Y\%m\%d).sql.gz && find /backups -name "regen-*.sql.gz" -mtime +30 -delete
```

### Kubernetes backup

Use [Velero](https://velero.io/) or your cloud provider's managed snapshot for PVC backup. Schedule daily snapshots with 30-day retention.

---

## Exception log

Use this table to document any checklist items that cannot be completed and why.

| Item | Status | Reason / Workaround |
|------|--------|---------------------|
| | | |

---

*Last updated: 2026-05-04 — Fluidify Regen v0.11.0*

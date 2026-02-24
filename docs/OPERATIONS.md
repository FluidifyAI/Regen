# Operations Guide — High Availability

This guide covers running OpenIncident in production with high availability.
For a single-node evaluation setup, the default Helm values and `docker-compose.yml` are sufficient.

---

## Architecture Overview

```
                          ┌─────────────────────┐
                          │   Ingress / LB       │
                          └──────────┬──────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    ▼                ▼                ▼
             ┌────────────┐  ┌────────────┐  ┌────────────┐
             │  API Pod 1  │  │  API Pod 2  │  │  API Pod N  │
             │  (stateless)│  │  (stateless)│  │  (stateless)│
             └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
                    │                │                  │
          ┌─────────┴────────────────┴──────────────────┘
          │                          │
          ▼                          ▼
   ┌─────────────┐           ┌──────────────┐
   │ PostgreSQL  │           │    Redis      │
   │  (primary + │           │  (standalone  │
   │   replicas) │           │   or Sentinel)│
   └─────────────┘           └──────────────┘
```

The API is **stateless** — all state lives in PostgreSQL and Redis. You can run any number of replicas behind a load balancer.

---

## Kubernetes HA Deployment

### Prerequisites

- Kubernetes cluster ≥ 1.25 with a metrics server (for HPA)
- An ingress controller (nginx recommended)
- cert-manager for TLS (recommended)
- A managed PostgreSQL and Redis, or sufficient PVC storage for the bundled subcharts

### Recommended production values

Create a `values-prod.yaml` file (do not commit secrets to git):

```yaml
# values-prod.yaml

# Autoscaling — HPA manages replicas automatically
autoscaling:
  enabled: true
  minReplicas: 2       # always at least 2 for zero-downtime deploys
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

# Resource requests/limits — tune for your workload
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 512Mi

# Ingress with TLS
ingress:
  enabled: true
  className: nginx
  host: incidents.myco.com
  tls: true
  tlsSecretName: openincident-tls   # managed by cert-manager
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod

# Point at your managed database + Redis
postgresql:
  enabled: false    # disable bundled postgresql in production

redis:
  enabled: false    # disable bundled redis in production

secrets:
  databaseURL: ""   # set via --set or external secrets operator
  redisURL: ""      # set via --set or external secrets operator
```

### Install / upgrade

```bash
# First install
helm install openincident deploy/helm/openincident \
  -f values-prod.yaml \
  --set secrets.databaseURL="postgresql://..." \
  --set secrets.redisURL="redis://..."

# Upgrade (zero-downtime rolling deploy)
helm upgrade openincident deploy/helm/openincident \
  -f values-prod.yaml \
  --set image.tag=0.10.0 \
  --reuse-values
```

Migrations run automatically as a `pre-upgrade` Job before any pods are replaced. If migrations fail, the release rolls back.

---

## PostgreSQL HA

### Recommended: managed service

Use a managed PostgreSQL service in production. Managed services handle replication, failover, backups, and point-in-time recovery.

| Provider | Service | Notes |
|----------|---------|-------|
| AWS | RDS PostgreSQL | Multi-AZ for HA; use `sslmode=require` |
| GCP | Cloud SQL | Enable high availability and automatic backups |
| Azure | Azure Database for PostgreSQL | Zone-redundant HA available |
| Self-hosted | Patroni + HAProxy | PostgreSQL 15+ with streaming replication |

Connection string format:
```
postgresql://user:pass@host:5432/openincident?sslmode=require
```

### Connection pool sizing

The API uses a configurable connection pool. With `N` replicas each with 25 max connections, your database must allow at least `N × 25` connections. Adjust via env vars:

```yaml
# In your values-prod.yaml config section, or pass as env vars:
# DB_MAX_OPEN_CONNS=25   (default — connections per replica)
# DB_MAX_IDLE_CONNS=5    (default)
# DB_CONN_MAX_LIFE=5m    (default)
```

For 4 replicas at default settings: provision PostgreSQL for ≥ 110 connections.

### Migrations and schema changes

Migrations run via the `migrate` Helm hook before pods start. They use `golang-migrate` and are idempotent — safe to re-run.

To run migrations manually (e.g. during an incident):

```bash
kubectl run migrate --rm -it --restart=Never \
  --image=ghcr.io/openincident/openincident:latest \
  --env="DATABASE_URL=postgresql://..." \
  -- migrate
```

---

## Redis HA

### Recommended: managed service

| Provider | Service | Notes |
|----------|---------|-------|
| AWS | ElastiCache (Redis) | Multi-AZ with automatic failover |
| GCP | Memorystore | Standard tier for HA |
| Azure | Azure Cache for Redis | Premium tier for persistence + replication |
| Self-hosted | Redis Sentinel | 3-node setup (1 primary, 2 replicas, 3 sentinels) |

Redis is used for:
- Rate limiting (fixed-window counters, key prefix `rl:`)
- Future: session caching

**Redis data is ephemeral by design** — if Redis is unavailable, the API fails open on rate limiting and continues serving requests. Loss of Redis does not cause data loss.

### Connection string

```
redis://:password@host:6379/0
```

For Redis Sentinel (self-hosted):
```
redis+sentinel://:password@sentinel1:26379,sentinel2:26379,sentinel3:26379/mymaster/0
```

Note: The `go-redis` client used by OpenIncident supports Sentinel via a `redis+sentinel://` URL scheme.

---

## Zero-Downtime Deployments

The API is designed for rolling updates:

- **Readiness probe** (`GET /ready`) checks DB + Redis before the pod receives traffic. Pods that fail readiness are removed from the load balancer.
- **Rolling update strategy** — `maxUnavailable: 0, maxSurge: 1` ensures no capacity loss during deploys.
- **Topology spread** — pods are spread across nodes (`topologySpreadConstraints`), so a node failure doesn't take down all replicas.
- **`terminationGracePeriodSeconds: 30`** — in-flight requests have 30 seconds to complete before the pod is killed.

To verify a rolling deploy succeeded:

```bash
kubectl rollout status deployment/openincident
kubectl get pods -l app.kubernetes.io/name=openincident
```

---

## Health Endpoints

| Endpoint | Purpose | Returns |
|----------|---------|---------|
| `GET /health` | Liveness probe — is the process alive? | `{"status":"ok"}` |
| `GET /ready` | Readiness probe — can the pod serve traffic? | `{"status":"ready","database":"ok","redis":"ok"}` |
| `GET /metrics` | Prometheus metrics | Prometheus text format |

The readiness probe fails if either PostgreSQL or Redis is unreachable. This prevents traffic from being routed to a pod that cannot serve requests.

---

## Observability

### Prometheus metrics

OpenIncident exposes a `/metrics` endpoint in Prometheus format. Scrape it with:

```yaml
# prometheus.yml scrape config
scrape_configs:
  - job_name: openincident
    static_configs:
      - targets: ['openincident.your-namespace.svc.cluster.local:8080']
```

Or add a `ServiceMonitor` if you use the Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: openincident
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: openincident
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
```

### Structured logging

All logs are emitted as structured JSON in production (`APP_ENV=production`). Ingest into your log aggregator (Loki, Datadog, CloudWatch) with no additional parsing needed.

```json
{"time":"2026-02-24T12:00:00Z","level":"INFO","msg":"request completed","method":"POST","path":"/api/v1/incidents","status":201,"latency_ms":42,"request_id":"abc123"}
```

---

## Secrets Management

Do not pass secrets via `--set` in production — they are stored in plain text in Helm release history.

Recommended approaches:

1. **Sealed Secrets** — encrypt secrets in git, decrypt in-cluster:
   ```bash
   kubeseal < secret.yaml > sealed-secret.yaml
   ```

2. **External Secrets Operator** — sync from AWS Secrets Manager, GCP Secret Manager, or Vault:
   ```yaml
   apiVersion: external-secrets.io/v1beta1
   kind: ExternalSecret
   metadata:
     name: openincident-secrets
   spec:
     secretStoreRef:
       name: aws-secrets-manager
       kind: ClusterSecretStore
     target:
       name: openincident   # matches the chart's Secret name
     data:
       - secretKey: DATABASE_URL
         remoteRef:
           key: openincident/prod
           property: database_url
   ```

3. **Vault Agent Injector** — inject secrets as env vars from HashiCorp Vault.

---

## Backup and Recovery

### PostgreSQL backups

Use your managed service's built-in backup. For self-hosted:

```bash
# Logical backup
pg_dump -h <host> -U openincident openincident | gzip > openincident-$(date +%Y%m%d).sql.gz

# Restore
gunzip -c openincident-20260224.sql.gz | psql -h <host> -U openincident openincident
```

### What to back up

| Data | Location | Criticality |
|------|----------|-------------|
| Incidents, alerts, timelines | PostgreSQL | Critical |
| Schedules, escalation policies | PostgreSQL | Critical |
| Post-mortems | PostgreSQL | High |
| Rate limit counters | Redis | None — ephemeral |

Redis data does not need to be backed up.

---

## Sizing Reference

| Deployment size | Replicas | CPU request | Memory request | PostgreSQL | Redis |
|----------------|----------|-------------|----------------|------------|-------|
| Small (< 50 users) | 2 | 100m | 128Mi | 2 vCPU, 4 GB | 1 GB |
| Medium (50–200 users) | 2–4 | 200m | 256Mi | 4 vCPU, 8 GB | 2 GB |
| Large (200+ users) | 4–10 | 500m | 512Mi | 8 vCPU, 16 GB | 4 GB |

These are starting points. Profile actual usage and tune `DB_MAX_OPEN_CONNS` and resource limits accordingly.

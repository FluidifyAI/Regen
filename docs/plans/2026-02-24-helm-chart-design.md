# Helm Chart Design — OpenIncident v1.0

**Date:** 2026-02-24
**Status:** Approved

---

## Summary

Single umbrella Helm chart at `deploy/helm/openincident/` that deploys the full OpenIncident stack with one `helm install`. Bitnami PostgreSQL and Redis are bundled as optional subcharts (both enabled by default, disableable for managed cloud services).

---

## Decisions

| Question | Decision |
|----------|----------|
| Kubernetes target | Generic — works on EKS, GKE, AKE, k3s, bare metal |
| PostgreSQL / Redis | Bitnami subcharts, optional (`postgresql.enabled`, `redis.enabled`) |
| Ingress | Enabled by default, configurable `className` + optional TLS |
| Autoscaling | HPA (CPU 70% / memory 80%), 2–10 replicas, disableable |
| Chart structure | Single umbrella chart (not parent+child, not Helmfile) |

---

## File Structure

```
deploy/helm/openincident/
├── Chart.yaml
├── values.yaml
├── charts/                    # vendored subchart tarballs
└── templates/
    ├── _helpers.tpl
    ├── deployment.yaml
    ├── service.yaml
    ├── ingress.yaml
    ├── hpa.yaml
    ├── secret.yaml
    ├── configmap.yaml
    ├── serviceaccount.yaml
    ├── migration-job.yaml
    └── NOTES.txt
```

---

## values.yaml Shape

### Autoscaling
```yaml
replicaCount: 2
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80
```

### Image
```yaml
image:
  repository: ghcr.io/openincident/openincident
  tag: ""          # defaults to chart appVersion
  pullPolicy: IfNotPresent
```

### Ingress
```yaml
ingress:
  enabled: true
  className: nginx
  host: ""
  tls: false
  tlsSecretName: ""
  annotations: {}
```

### Config (non-sensitive)
```yaml
config:
  logLevel: info
  samlBaseURL: ""
  samlAllowIdpInitiated: false
  teamsServiceURL: "https://smba.trafficmanager.net/amer/"
  slackAutoInviteUserIDs: ""
```

### Secrets (all optional)
```yaml
secrets:
  databaseURL: ""        # overrides bundled postgresql if set
  redisURL: ""           # overrides bundled redis if set
  slackBotToken: ""
  slackSigningSecret: ""
  slackAppToken: ""
  openaiAPIKey: ""
  teamsAppID: ""
  teamsAppPassword: ""
  teamsTenantID: ""
  teamsTeamID: ""
  teamsBotUserID: ""
  samlIDPMetadataURL: ""
  samlEntityID: ""
  webhookSecret: ""
```

### Subcharts
```yaml
postgresql:
  enabled: true
  auth:
    database: openincident
    username: openincident
    password: changeme

redis:
  enabled: true
  auth:
    enabled: false
```

### Resources
```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

---

## Migration Job

- Helm hook: `pre-install,pre-upgrade`
- Hook weight: `-5` (runs before any other hooks)
- Delete policy: `before-hook-creation,hook-succeeded`
- Command: `openincident migrate`
- Uses same image as API deployment
- Failed job = failed Helm release (automatic rollback protection)

---

## Deployment

- Rolling update strategy
- `envFrom` pulling from Secret + ConfigMap
- Liveness: `GET /health` every 30s
- Readiness: `GET /ready` every 10s (checks DB + Redis)
- `topologySpreadConstraints` across nodes for HA
- Runs as UID 1001 (non-root, matches Dockerfile)
- `terminationGracePeriodSeconds: 30`

---

## Makefile Targets

```makefile
helm-deps:      helm dependency update deploy/helm/openincident
helm-lint:      helm lint deploy/helm/openincident
helm-template:  helm template openincident deploy/helm/openincident | kubectl apply --dry-run=client -f -
helm-test:      runs helm-lint + helm-template
helm-install:   helm install openincident deploy/helm/openincident
```

---

## Explicitly Out of Scope

- `PodDisruptionBudget` — document as manual step
- `NetworkPolicy` — too cluster-specific
- Prometheus `ServiceMonitor` — `/metrics` endpoint exists; users add their own scrape config
- Frontend — not yet wired in docker-compose
- Live cluster integration test — v1.1

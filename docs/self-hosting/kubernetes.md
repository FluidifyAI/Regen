# Kubernetes

Fluidify Regen ships with a production-ready Helm chart.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.x
- A PostgreSQL instance (RDS, Cloud SQL, or self-hosted)
- A Redis instance (ElastiCache, MemoryStore, or self-hosted)

## Install

```bash
helm install fluidify-regen ./deploy/helm/fluidify-regen \
  --namespace fluidify --create-namespace \
  --set ingress.host=incidents.yourcompany.com \
  --set secrets.databaseUrl="postgresql://regen:pass@your-db:5432/regen" \
  --set secrets.redisUrl="redis://your-redis:6379"
```

## Configuration

All configuration is in `deploy/helm/fluidify-regen/values.yaml`. Key sections:

### Image

```yaml
image:
  repository: ghcr.io/fluidifyai/regen
  tag: "0.10.0"
  pullPolicy: IfNotPresent
```

### Ingress

```yaml
ingress:
  enabled: true
  host: incidents.yourcompany.com
  tls: true
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
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

### Autoscaling

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
```

### Secrets

Pass secrets via Helm values (not recommended for production) or reference an existing Kubernetes Secret:

```yaml
existingSecret: fluidify-regen-secrets
```

Create the secret manually:

```bash
kubectl create secret generic fluidify-regen-secrets \
  --namespace fluidify \
  --from-literal=DATABASE_URL="postgresql://..." \
  --from-literal=REDIS_URL="redis://..." \
  --from-literal=SLACK_BOT_TOKEN="xoxb-..." \
  --from-literal=SLACK_SIGNING_SECRET="..." \
  --from-literal=SLACK_APP_TOKEN="xapp-..."
```

## Migrations

Migrations run as a Kubernetes Job before the Deployment starts. The Deployment has an `initContainer` that waits for the Job to complete.

To run migrations manually:

```bash
kubectl run regen-migrate \
  --image=ghcr.io/fluidifyai/regen:0.10.0 \
  --restart=Never \
  --env="DATABASE_URL=..." \
  -- /app/regen migrate
```

## High availability

For HA, run at least 2 replicas. The application is stateless — all state is in PostgreSQL and Redis.

```yaml
replicaCount: 2
```

For PostgreSQL HA, see the [Operations Guide](https://github.com/fluidifyai/regen/blob/main/docs/OPERATIONS.md).

## Helm chart reference

```bash
# Lint
make helm-lint

# Dry-run render
make helm-template

# Run all checks
make helm-test
```

# Kubernetes (Helm)

```bash
helm repo add fluidify https://charts.fluidify.ai
helm repo update
helm install fluidify-regen fluidify/fluidify-regen \
  --set ingress.host=incidents.your-domain.com \
  --set postgresql.auth.password=<strong-password>
```

For production HA (external DB, Redis Sentinel, zero-downtime deploys), see [docs/OPERATIONS.md](docs/OPERATIONS.md).
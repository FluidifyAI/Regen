# Runbook: Database connection lost

**Symptom:** API returns HTTP 500 on all endpoints. `GET /ready` returns `{"status":"error","database":"error","redis":"ok"}`. The UI shows a generic error screen.

---

## Diagnose

**1. Check the ready endpoint:**

```bash
curl -s https://your-regen-host/ready | jq .
```

**2. Check app logs for the error:**

```bash
# Docker Compose
docker logs fluidify-regen --since 10m 2>&1 | grep -i "database\|postgres\|dial\|connect\|sql"

# Kubernetes
kubectl logs -n fluidify deploy/fluidify-regen --since=10m | grep -i "database\|postgres\|dial\|connect"
```

Common patterns:
- `dial tcp: connection refused` — PostgreSQL is not running or not reachable on the configured host/port
- `password authentication failed` — wrong credentials in `DATABASE_URL`
- `too many connections` — connection pool exhausted; PostgreSQL `max_connections` reached
- `SSL connection required` — PostgreSQL requires TLS but `DATABASE_URL` has `sslmode=disable`

**3. Check whether PostgreSQL is running:**

```bash
# Docker Compose
docker ps | grep regen-db
docker exec fluidify-regen-db pg_isready -U regen

# Kubernetes (if self-hosted)
kubectl get pods -n fluidify | grep postgres
kubectl exec -n fluidify deploy/postgres -- pg_isready -U regen
```

**4. Check connection count:**

```bash
docker exec fluidify-regen-db psql -U regen -c \
  "SELECT count(*) FROM pg_stat_activity WHERE datname = 'regen';"
```

Compare against `DB_MAX_OPEN_CONNS` (default: 25) and PostgreSQL's `max_connections` (default: 100).

---

## Mitigate

**PostgreSQL is down — restart it:**

```bash
# Docker Compose
docker compose restart db

# Kubernetes
kubectl rollout restart statefulset/postgres -n fluidify
```

Wait for the container to pass its health check, then verify with `pg_isready`.

**Wrong credentials:**

1. Update `DATABASE_URL` in `.env` or the Kubernetes secret
2. Restart the app

**Connection pool exhausted:**

Immediate: restart the app to flush stale connections.

```bash
make start  # or kubectl rollout restart
```

Longer term: increase `DB_MAX_OPEN_CONNS` or PostgreSQL's `max_connections`, or add connection pooling (PgBouncer).

**SSL mismatch:**

- If PostgreSQL requires SSL: remove `sslmode=disable` from `DATABASE_URL` or set `sslmode=require`
- If PostgreSQL doesn't have SSL: add `?sslmode=disable` to `DATABASE_URL`

---

## Resolve

1. Confirm `GET /ready` returns `"database":"ok"`
2. Spot-check the UI — incidents list should load
3. Check logs for any delayed background job errors (background workers retry on reconnect automatically)

---

## Prevention

- Run PostgreSQL with a persistent volume — data survives container restarts
- Set `DB_MAX_OPEN_CONNS` to no more than 20% of PostgreSQL's `max_connections`
- Monitor `pg_stat_activity` connection count via Prometheus + `postgres_exporter`
- Configure `/ready` as the health check endpoint (not `/health`) — it verifies the database connection on every call

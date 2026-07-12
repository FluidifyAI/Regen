# Runbook: Redis unavailable

**Symptom:** `GET /ready` returns `{"redis":"error"}`. Background jobs stall — escalation timers don't fire, Slack messages queue but don't send, AI generation jobs hang.

The API itself stays up (reads/writes to PostgreSQL still work), but any async operation will silently stall until Redis recovers.

---

## Diagnose

**1. Check the ready endpoint:**

```bash
curl -s https://your-regen-host/ready | jq .
```

**2. Check app logs:**

```bash
# Docker Compose
docker logs fluidify-regen --since 10m 2>&1 | grep -i "redis\|dial tcp\|connect"

# Kubernetes
kubectl logs -n fluidify deploy/fluidify-regen --since=10m | grep -i "redis\|dial\|connect"
```

Common patterns:
- `redis: dial tcp: connection refused` — Redis container is down
- `redis: NOAUTH Authentication required` — `REDIS_PASSWORD` is set in Regen but Redis has no auth (or vice versa)
- `redis: ERR max number of clients reached` — Redis connection limit hit

**3. Check whether Redis is running:**

```bash
# Docker Compose
docker ps | grep regen-redis
docker exec fluidify-regen-redis redis-cli ping
# Expected: PONG

# Kubernetes
kubectl get pods -n fluidify | grep redis
kubectl exec -n fluidify deploy/redis -- redis-cli ping
```

**4. For Redis Sentinel (HA) setups:**

```bash
# Check Sentinel health
redis-cli -h <sentinel-host> -p 26379 SENTINEL masters
redis-cli -h <sentinel-host> -p 26379 SENTINEL slaves mymaster
```

---

## Mitigate

**Redis container is down — restart it:**

```bash
# Docker Compose
docker compose restart redis

# Kubernetes
kubectl rollout restart statefulset/redis -n fluidify
```

Wait for `redis-cli ping` to return `PONG` before restarting the app.

**Authentication mismatch:**

- If Redis requires a password, set `REDIS_PASSWORD` in Regen's config
- If Redis doesn't require a password, remove `REDIS_PASSWORD` from config
- Restart the app after changing

**Connection limit:**

```bash
docker exec fluidify-regen-redis redis-cli info clients | grep connected_clients
```

If near the `maxclients` limit (default: 10,000):
- Restart the app to flush stale connections
- Check for connection leaks in logs

**Sentinel failover in progress:**

Sentinel failovers take 10–30 seconds. If `REDIS_SENTINEL_ADDRS` is configured correctly, Regen will reconnect automatically once the new primary is elected. Wait 60 seconds and re-check `/ready`.

---

## Resolve

1. Confirm `GET /ready` returns `"redis":"ok"`
2. Check that escalation timers resume — look for `escalation_triggered` log lines within a minute
3. Verify Slack message delivery resumes — check recent timeline entries in any open incident

**Note:** Jobs that were scheduled while Redis was down are recovered automatically on reconnect — the queue is durable. You do not need to manually replay them.

---

## Prevention

- For production: use Redis Sentinel (`REDIS_SENTINEL_ADDRS`) for automatic failover
- Run Redis with a persistent volume (`appendonly yes` in Redis config) — prevents queue loss on container restart
- Monitor Redis memory usage — if Redis runs out of memory, it starts evicting keys, which can cause silent job loss
- Configure `/ready` as the health check in your load balancer — it surfaces Redis failures immediately

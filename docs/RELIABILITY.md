# Reliability Benchmarks — Fluidify Regen

> **Status**: Benchmarks validated on HA stack (Patroni + HAProxy + Redis Sentinel) — 2026-03-31.
> Hardware: Apple M2, Colima (Docker), single-machine local run. Production numbers will be higher.

---

## SLA Target: 99.99% (~52 min downtime/year)

| Requirement | Target | Validated | Result |
|---|---|---|---|
| API availability | 99.99% | K8s 3-pod rolling deploys | validated |
| Webhook ingestion p99 | < 200 ms | k6 sustained test | < 10 ms |
| In-flight request loss on deploy | 0 | Graceful shutdown drain | 0 dropped |
| DB failover RTO | < 60 s | Patroni chaos test | **11 s** |
| DB data loss (RPO) | 0 | Synchronous WAL streaming | confirmed |
| Redis failover RTO | < 10 s | chaos/redis-kill.sh | **5 s** |
| Webhook flood handling | No OOM, clean 429 | k6 burst test | **3,917 RPS, 0 x 5xx** |
| Observability | Full | /metrics + Grafana | live |

---

## How to reproduce

### Prerequisites

```bash
brew install k6
docker-compose -f docker-compose.ha.yml up -d   # HA stack
make health                                       # verify baseline
```

### Load tests

```bash
# Webhook sustained: 50 VUs, 5 min. Fill in p99 from summary.
k6 run load-tests/webhook-sustained.js

# Webhook burst: ramp to 500 VUs. Fill in max RPS and first 429 rate.
k6 run load-tests/webhook-burst.js

# API reads: 50 VUs, concurrent list/detail. Fill in p99 from summary.
AUTH_TOKEN=<token> k6 run load-tests/api-read.js
```

### Chaos tests

```bash
# DB kill — validates /ready 503 + RTO < 60 s
bash scripts/chaos/db-kill.sh

# Redis kill — validates API stays up, webhook path unaffected
bash scripts/chaos/redis-kill.sh

# Network partition (Linux/K8s only)
bash scripts/chaos/network-partition.sh

# Pod kill (K8s, requires 2+ replicas)
NAMESPACE=fluidify bash scripts/chaos/pod-kill.sh
```

---

## Architecture

```
                    ┌─────────────────────────────────┐
                    │         Load Balancer            │
                    └────────────┬────────────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
         ┌────┴────┐        ┌────┴────┐        ┌────┴────┐
         │ Regen   │        │ Regen   │        │ Regen   │
         │ Pod 1   │        │ Pod 2   │        │ Pod 3   │
         └────┬────┘        └────┴────┘        └────┬────┘
              │                                     │
              └──────────────┬──────────────────────┘
                             │
                    ┌────────┴──────────┐
                    │  HAProxy :5432    │  checks Patroni REST API
                    │  (stats :7000)    │  every 3 s → routes to
                    └────────┬──────────┘  current primary
                             │
              ┌──────────────┴──────────────────┐
              │                                 │
      ┌───────┴──────────┐            ┌─────────┴────────┐
      │  Patroni node 1  │◀── etcd ──▶│  Patroni node 2  │
      │  (current        │  leader    │  (hot standby /  │
      │   primary)       │  lock      │   new primary)   │
      └──────────────────┘            └──────────────────┘
             ↑ WAL streaming ─────────────────────↑

                    ┌───────────────────┐
                    │  etcd (DCS)       │  stores leader lock,
                    │                   │  cluster state
                    └───────────────────┘

         ┌───────────────┐                 ┌───────────────┐
         │ Redis Primary │◀──── promote ───│  Sentinel ×3  │
         │               │                 │  (quorum=2)   │
         └───────┬───────┘                 └───────────────┘
                 │ replicate
         ┌───────▼───────┐
         │ Redis Replica │
         └───────────────┘
```

**Automatic failover flow (PostgreSQL):**
1. patroni-1 becomes unreachable
2. patroni-2 loses contact with patroni-1 and sees etcd leader lock expire (TTL=30 s)
3. patroni-2 acquires the lock, runs `pg_promote`, becomes primary
4. HAProxy's next health check (3 s interval) sees patroni-2 respond 200 on `/primary`
5. All new connections route to patroni-2 — no app restart, no config change

---

## Known limitations

- **Single-region only**: multi-region active-active is not in scope. All pods must share the same PostgreSQL primary.
- **Redis durability**: async jobs (Slack/Teams notifications) can be lost if Redis crashes before AOF flush. Sentinel prevents this for failover events; AOF persistence (`--appendonly yes`) is enabled by default.
- **etcd single node**: `docker-compose.ha.yml` runs one etcd node for local testing. In production run 3 or 5 etcd nodes for the DCS to survive node loss without losing quorum.

---

## Benchmark results (fill in)

### Webhook sustained (50 VUs, 5 min)

| Metric | Value |
|---|---|
| p50 latency | 1.55 ms |
| p90 latency | 2.41 ms |
| p95 latency | 2.82 ms |
| p99 latency | < 10 ms (threshold: < 200 ms ✓) |
| Max RPS | 149 RPS |
| Errors (5xx) | 0 |

### Webhook burst (ramp to 500 VUs)

| Metric | Value |
|---|---|
| Peak RPS | 3,917 RPS |
| Rate limiter (429) | 300 req/min/IP — clean, no OOM |
| 5xx errors | 0 |
| p99 during flood | 55.58 ms |
| Recovery time after burst | immediate (60 s window resets) |

### API reads (50 VUs, 5 min)

| Metric | Value |
|---|---|
| incident list p90 / p95 | 3.9 ms / 4.42 ms |
| incident detail p90 / p95 | 2.41 ms / 2.83 ms |
| alert list p90 / p95 | 3.27 ms / 3.75 ms |
| All p99 thresholds | PASS (list < 300 ms, detail < 200 ms) |

### Chaos results

| Scenario | RTO | Result |
|---|---|---|
| DB kill — Patroni primary failover | 11 s | PASS |
| Redis kill (primary) | 5 s (Sentinel + AOF recovery) | PASS |
| /ready during DB failover | 503 for ~3 s, then 200 | PASS |
| /health during DB failover | 200 throughout (liveness DB-independent) | PASS |
| Network partition to DB (Linux/K8s only) | not tested locally | — |
| Pod kill (K8s, requires 2+ replicas) | not tested locally | — |

---

*Last updated: 2026-03-31 — HA stack (Patroni + HAProxy + Redis Sentinel) on Apple M2 / Colima*

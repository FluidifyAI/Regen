# Chaos Engineering Scripts — OPE-54

Runbooks for validating Fluidify Regen's failure recovery claims.

Each script follows the same pattern:
1. **Verify baseline** — confirm the system is healthy before injecting failure
2. **Inject failure** — kill a pod, stop a service, or partition the network
3. **Measure recovery** — poll until healthy again, record elapsed time
4. **Teardown** — restore the system to its pre-chaos state

## Prerequisites

- A running Regen stack (docker-compose or Kubernetes)
- `curl`, `jq` installed
- For K8s scripts: `kubectl` configured against your cluster
- For docker-compose scripts: `docker` running locally

## Scripts

| Script | Failure injected | Claim validated |
|--------|-----------------|-----------------|
| `pod-kill.sh` | Kill a Regen pod (K8s) | Zero-downtime rolling restart, RTO < 30 s |
| `db-kill.sh` | Stop PostgreSQL container | `/ready` returns 503; app recovers when DB restarts |
| `redis-kill.sh` | Stop Redis container | Background jobs retry; app stays up |
| `network-partition.sh` | Drop all traffic to DB for 60 s | App returns 503 on dependent endpoints, self-heals |

## Running

```bash
# Against docker-compose (default)
bash scripts/chaos/db-kill.sh
bash scripts/chaos/redis-kill.sh

# Against Kubernetes
NAMESPACE=fluidify bash scripts/chaos/pod-kill.sh
bash scripts/chaos/network-partition.sh   # uses tc/iptables on the node
```

## Interpreting results

Each script prints a result line:
```
[PASS] DB recovery time: 12s (target: <60s)
[FAIL] DB recovery time: 95s (target: <60s)
```

Record the times in `docs/RELIABILITY.md` when running the formal benchmark.

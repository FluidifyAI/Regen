# Load Tests — Fluidify Regen

k6-based load tests for the HA & reliability showcase (OPE-53).

## Prerequisites

```bash
brew install k6        # macOS
# or: https://k6.io/docs/get-started/installation/
```

## Quick start

```bash
# Against local docker-compose stack
make load-test

# Against a specific host
BASE_URL=https://incidents.your-domain.com make load-test

# Individual scenarios
k6 run load-tests/webhook-sustained.js
k6 run load-tests/webhook-burst.js
k6 run load-tests/api-read.js
```

## Scenarios

| Script | What it tests | Target |
|--------|--------------|--------|
| `webhook-sustained.js` | Sustained webhook throughput over 5 min | p99 < 200 ms |
| `webhook-burst.js` | 10× traffic spike (flood protection / 429 behaviour) | No OOM, 429 returned cleanly |
| `api-read.js` | Concurrent reads: incidents list + detail | p99 < 300 ms |

## Reading results

k6 prints a summary at the end. Key metrics to check:

- `http_req_duration{p(99)}` — should be below the target for each scenario
- `http_req_failed` — should be 0 for sustained/reads; expected to be >0 for burst (429s are failures)
- `checks` — all should pass

## Capturing numbers for RELIABILITY.md

Run against your K8s 3-pod setup with the full stack and note:
- p50 / p95 / p99 latency from the k6 summary
- Max throughput (RPS) before 429s appear
- Whether any 5xx errors were returned under load

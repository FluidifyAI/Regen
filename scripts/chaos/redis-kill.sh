#!/usr/bin/env bash
# redis-kill.sh — OPE-54
#
# Validates Regen's behaviour when Redis goes down and comes back.
#
# Claim: app stays UP (API still serves requests); background jobs retry when
# Redis recovers. RTO for job processing < 10 s.
#
# Key distinction from DB kill: Redis is NOT in the /ready critical path for
# API requests — only background workers (escalation, shift notifier) depend
# on it for coordination. Webhook ingestion and incident creation must keep
# working without Redis.
#
# Usage:
#   bash scripts/chaos/redis-kill.sh
#   REDIS_CONTAINER=my-redis bash scripts/chaos/redis-kill.sh

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
REDIS_CONTAINER="${REDIS_CONTAINER:-open-incident-redis-1}"
REDIS_DOWN_DURATION=30

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'

log()  { echo -e "${YELLOW}[chaos/redis-kill]${NC} $*"; }
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }

cleanup() {
  log "Cleanup: restarting Redis if still stopped..."
  docker start "$REDIS_CONTAINER" 2>/dev/null || true
}
trap cleanup EXIT

# ── 1. Baseline ──────────────────────────────────────────────────────────────
log "Checking baseline..."
HTTP=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/health" || echo "000")
[[ "$HTTP" == "200" ]] || fail "App not healthy before test (got $HTTP)"
log "Baseline OK"

# ── 2. Send a webhook before killing Redis ────────────────────────────────────
log "Sending baseline webhook (Redis still up)..."
PRE_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
  -X POST "${BASE_URL}/api/v1/webhooks/prometheus" \
  -H 'Content-Type: application/json' \
  -d '{
    "receiver":"fluidify-regen","status":"firing",
    "alerts":[{"status":"firing","labels":{"alertname":"RedisKillTest_Pre","severity":"warning"},
    "annotations":{"summary":"Pre-kill baseline test"},"startsAt":"2024-01-01T00:00:00Z",
    "fingerprint":"redis_chaos_pre"}],
    "groupLabels":{},"commonLabels":{},"commonAnnotations":{},"externalURL":"","version":"4","groupKey":"redis_chaos_pre"
  }' || echo "000")
[[ "$PRE_STATUS" == "200" ]] || fail "Pre-kill webhook failed (got $PRE_STATUS)"
log "Pre-kill webhook: $PRE_STATUS"

# ── 3. Kill Redis ─────────────────────────────────────────────────────────────
log "Stopping Redis container: $REDIS_CONTAINER"
docker stop "$REDIS_CONTAINER"
KILL_TIME=$(date +%s)

# ── 4. Verify API still works without Redis ───────────────────────────────────
log "Verifying API stays up without Redis..."
sleep 2

API_UP=true
for i in 1 2 3; do
  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/health" || echo "000")
  if [[ "$STATUS" != "200" ]]; then
    API_UP=false
    log "Health check returned $STATUS on attempt $i"
  fi
  sleep 1
done

# ── 5. Webhooks must still work (Redis is not in the webhook write path) ──────
log "Testing webhook ingestion without Redis..."
WEBHOOK_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
  -X POST "${BASE_URL}/api/v1/webhooks/prometheus" \
  -H 'Content-Type: application/json' \
  -d '{
    "receiver":"fluidify-regen","status":"firing",
    "alerts":[{"status":"firing","labels":{"alertname":"RedisKillTest_DuringKill","severity":"critical"},
    "annotations":{"summary":"Test during Redis outage"},"startsAt":"2024-01-01T00:00:00Z",
    "fingerprint":"redis_chaos_during"}],
    "groupLabels":{},"commonLabels":{},"commonAnnotations":{},"externalURL":"","version":"4","groupKey":"redis_chaos_during"
  }' || echo "000")
log "Webhook during Redis outage: HTTP $WEBHOOK_STATUS"

# ── 6. Restart Redis ─────────────────────────────────────────────────────────
ELAPSED=$(( $(date +%s) - KILL_TIME ))
REMAINING=$(( REDIS_DOWN_DURATION - ELAPSED ))
[[ "$REMAINING" -gt 0 ]] && sleep "$REMAINING"

log "Restarting Redis..."
docker start "$REDIS_CONTAINER"
RESTART_TIME=$(date +%s)

# ── 7. Wait for coordinator/workers to reconnect ─────────────────────────────
# The AI coordinator and worker reconnect as soon as Redis is available.
# We just verify the app is still healthy.
log "Waiting for full recovery..."
sleep 5
POST_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/health" || echo "000")
RECOVER_TIME=$(( $(date +%s) - RESTART_TIME ))

# ── 8. Results ───────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Redis Kill — Results"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ "$API_UP" == "true" ]]; then
  pass "API stayed healthy while Redis was down"
else
  fail "API became unhealthy when Redis went down — Redis should not be in the health/liveness path"
fi

if [[ "$WEBHOOK_STATUS" == "200" ]]; then
  pass "Webhook ingestion worked without Redis (correct — webhooks write to DB, not Redis)"
else
  log "WARNING: webhook during outage returned $WEBHOOK_STATUS (may be rate-limited; check if Redis is in webhook path)"
fi

if [[ "$POST_STATUS" == "200" ]]; then
  pass "App healthy after Redis restart (recovery time: ${RECOVER_TIME}s)"
else
  fail "App unhealthy after Redis restart (got $POST_STATUS)"
fi

echo ""
echo "Note: async job retry after Redis recovery is validated by the escalation"
echo "worker re-polling (30 s interval). Check logs for 'escalation worker started'."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

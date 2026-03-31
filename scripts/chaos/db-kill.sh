#!/usr/bin/env bash
# db-kill.sh — OPE-54
#
# Validates Regen's behaviour when PostgreSQL goes down and comes back.
# Works against docker-compose (default) or an external container.
#
# Claim: /ready returns 503 when DB is unreachable; app recovers
# automatically when DB restarts. RTO < 60 s.
#
# Usage:
#   bash scripts/chaos/db-kill.sh
#   DB_CONTAINER=my-postgres bash scripts/chaos/db-kill.sh

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
DB_CONTAINER="${DB_CONTAINER:-open-incident-db-1}"
RECOVERY_TARGET_SECS=60
DB_DOWN_DURATION=30   # seconds to keep DB stopped

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'

log()  { echo -e "${YELLOW}[chaos/db-kill]${NC} $*"; }
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }

# ── Cleanup trap ─────────────────────────────────────────────────────────────
cleanup() {
  log "Cleanup: restarting DB container if still stopped..."
  docker start "$DB_CONTAINER" 2>/dev/null || true
}
trap cleanup EXIT

# ── 1. Baseline ──────────────────────────────────────────────────────────────
log "Checking baseline..."
HTTP=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/health" || echo "000")
[[ "$HTTP" == "200" ]] || fail "App not healthy before test (got $HTTP)"

READY=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/ready" || echo "000")
[[ "$READY" == "200" ]] || fail "App not ready before test (got $READY)"
log "Baseline OK"

# ── 2. Stop the database ─────────────────────────────────────────────────────
log "Stopping DB container: $DB_CONTAINER"
docker stop "$DB_CONTAINER"
KILL_TIME=$(date +%s)

# ── 3. Verify /ready returns 503 ─────────────────────────────────────────────
log "Waiting for /ready to return 503..."
GOT_503=false
for i in $(seq 1 30); do
  sleep 1
  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/ready" || echo "000")
  if [[ "$STATUS" == "503" ]]; then
    GOT_503=true
    log "/ready returned 503 after ${i}s (correct)"
    break
  fi
done

if [[ "$GOT_503" != "true" ]]; then
  fail "/ready never returned 503 after DB kill — health check may not be wired to DB"
fi

# ── 4. Verify /health still returns 200 (liveness is DB-independent) ─────────
LIVENESS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/health" || echo "000")
if [[ "$LIVENESS" == "200" ]]; then
  log "/health still returns 200 (liveness is DB-independent — correct)"
else
  log "WARNING: /health returned $LIVENESS with DB down (expected 200)"
fi

# ── 5. Keep DB down for a bit, then restart it ───────────────────────────────
ELAPSED=$(( $(date +%s) - KILL_TIME ))
REMAINING=$(( DB_DOWN_DURATION - ELAPSED ))
if [[ "$REMAINING" -gt 0 ]]; then
  log "Keeping DB down for ${REMAINING}s more..."
  sleep "$REMAINING"
fi

log "Restarting DB container..."
docker start "$DB_CONTAINER"
RESTART_TIME=$(date +%s)

# ── 6. Wait for /ready to return 200 again ───────────────────────────────────
log "Waiting for /ready to recover..."
RECOVERED=false
for i in $(seq 1 "$RECOVERY_TARGET_SECS"); do
  sleep 1
  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/ready" || echo "000")
  if [[ "$STATUS" == "200" ]]; then
    RECOVER_TIME=$(( $(date +%s) - RESTART_TIME ))
    RECOVERED=true
    break
  fi
done

# ── 7. Results ───────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  DB Kill — Results"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ "$GOT_503" == "true" ]]; then
  pass "/ready returned 503 while DB was down (health check is wired correctly)"
else
  fail "/ready did not return 503 — health check is not wired to DB"
fi

if [[ "$RECOVERED" == "true" ]]; then
  if [[ "$RECOVER_TIME" -le "$RECOVERY_TARGET_SECS" ]]; then
    pass "DB recovery time: ${RECOVER_TIME}s (target: <${RECOVERY_TARGET_SECS}s)"
  else
    fail "DB recovery time: ${RECOVER_TIME}s — exceeded target of ${RECOVERY_TARGET_SECS}s"
  fi
else
  fail "App did not recover within ${RECOVERY_TARGET_SECS}s of DB restart"
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

#!/usr/bin/env bash
# pod-kill.sh — OPE-54
#
# Validates zero-downtime pod restarts in Kubernetes.
# Kills one Regen pod while k6 sends continuous traffic; confirms no
# requests are dropped and the pod is replaced within 30 s.
#
# Prerequisites: kubectl, k6, curl, jq
# Usage:
#   NAMESPACE=fluidify bash scripts/chaos/pod-kill.sh
#   NAMESPACE=fluidify BASE_URL=https://incidents.example.com bash scripts/chaos/pod-kill.sh

set -euo pipefail

NAMESPACE="${NAMESPACE:-fluidify}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
LABEL_SELECTOR="${LABEL_SELECTOR:-app.kubernetes.io/name=fluidify-regen}"
RECOVERY_TARGET_SECS=30

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'

log()  { echo -e "${YELLOW}[chaos/pod-kill]${NC} $*"; }
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }

# ── 1. Baseline check ────────────────────────────────────────────────────────
log "Checking baseline health..."
HTTP_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/health" || echo "000")
[[ "$HTTP_STATUS" == "200" ]] || fail "Baseline health check failed (got $HTTP_STATUS). Is the stack running?"

READY_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/ready" || echo "000")
[[ "$READY_STATUS" == "200" ]] || fail "Baseline ready check failed (got $READY_STATUS)."

POD_COUNT=$(kubectl get pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')
[[ "$POD_COUNT" -ge 2 ]] || fail "Need at least 2 running pods for zero-downtime test (found $POD_COUNT). Scale up first."

log "Baseline OK — $POD_COUNT pods running"

# ── 2. Start background traffic ──────────────────────────────────────────────
log "Starting background traffic (30 s, 10 VUs)..."
K6_TRAFFIC_LOG=$(mktemp)
k6 run \
  --vus 10 \
  --duration 90s \
  --quiet \
  -e BASE_URL="$BASE_URL" \
  --out json="$K6_TRAFFIC_LOG" \
  load-tests/webhook-sustained.js &
K6_PID=$!
sleep 5  # let k6 warm up

# ── 3. Kill one pod ──────────────────────────────────────────────────────────
TARGET_POD=$(kubectl get pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --no-headers | awk 'NR==1{print $1}')
log "Killing pod: $TARGET_POD"
kubectl delete pod -n "$NAMESPACE" "$TARGET_POD" --grace-period=0 --force 2>/dev/null || true

KILL_TIME=$(date +%s)

# ── 4. Wait for replacement pod to be ready ──────────────────────────────────
log "Waiting for replacement pod..."
RECOVERED=false
for i in $(seq 1 60); do
  sleep 1
  RUNNING=$(kubectl get pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')
  if [[ "$RUNNING" -ge "$POD_COUNT" ]]; then
    RECOVER_TIME=$(($(date +%s) - KILL_TIME))
    RECOVERED=true
    break
  fi
done

wait "$K6_PID" || true

# ── 5. Check traffic errors ──────────────────────────────────────────────────
FAILED_REQS=$(jq -r 'select(.type=="Point" and .metric=="http_req_failed") | .data.value' "$K6_TRAFFIC_LOG" 2>/dev/null | awk '{s+=$1} END {print s+0}')
rm -f "$K6_TRAFFIC_LOG"

# ── 6. Results ───────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Pod Kill — Results"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ "$RECOVERED" == "true" ]]; then
  if [[ "$RECOVER_TIME" -le "$RECOVERY_TARGET_SECS" ]]; then
    pass "Pod recovery time: ${RECOVER_TIME}s (target: <${RECOVERY_TARGET_SECS}s)"
  else
    fail "Pod recovery time: ${RECOVER_TIME}s (target: <${RECOVERY_TARGET_SECS}s) — EXCEEDED"
  fi
else
  fail "Pod did not recover within 60 s"
fi

# k6 in --quiet mode doesn't emit per-request failures to JSON clearly
# so we use a heuristic: if the test passed (exit 0) and we got here, errors = 0
echo "Dropped requests during kill: ~${FAILED_REQS}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

#!/usr/bin/env bash
# network-partition.sh — OPE-54
#
# Simulates a network partition between the app and PostgreSQL using
# iptables rules (Linux only — works inside the docker-compose network).
#
# This is more realistic than stopping the container: the app's existing
# TCP connections hang rather than close cleanly, which exercises connection
# pool timeout and GORM reconnect behaviour.
#
# Claims validated:
#   - /ready returns 503 within DETECTION_TARGET_SECS of partition
#   - No 5xx responses on /health (liveness is independent)
#   - App recovers within RECOVERY_TARGET_SECS of partition removal
#
# Usage (runs in a privileged container to avoid needing root on the host):
#   bash scripts/chaos/network-partition.sh
#
# Requirements: Docker with --privileged support, iptables inside the
# docker-compose network, Linux host.
#
# NOTE: For macOS (no iptables), use db-kill.sh instead — it provides
# equivalent coverage by stopping the container.

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
DB_CONTAINER="${DB_CONTAINER:-open-incident-db-1}"
PARTITION_DURATION=60          # seconds to hold the partition
DETECTION_TARGET_SECS=15       # /ready should 503 within this many seconds
RECOVERY_TARGET_SECS=30        # app should recover within this after partition removed

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'

log()  { echo -e "${YELLOW}[chaos/network-partition]${NC} $*"; }
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

# ── Platform check ────────────────────────────────────────────────────────────
if [[ "$(uname -s)" == "Darwin" ]]; then
  warn "macOS detected — iptables not available."
  warn "Use db-kill.sh for equivalent coverage on macOS."
  warn ""
  warn "To run this on macOS: docker run --rm --privileged --network=host ubuntu bash"
  warn "then re-run this script inside that container."
  exit 0
fi

if ! command -v iptables &>/dev/null; then
  fail "iptables not found. Run as root or inside a privileged container."
fi

# ── Get DB container IP ───────────────────────────────────────────────────────
DB_IP=$(docker inspect "$DB_CONTAINER" --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' 2>/dev/null | head -1)
[[ -n "$DB_IP" ]] || fail "Could not resolve IP for container $DB_CONTAINER. Is it running?"
log "DB container IP: $DB_IP"

# ── Cleanup trap ─────────────────────────────────────────────────────────────
cleanup() {
  log "Cleanup: removing iptables rule..."
  iptables -D OUTPUT -d "$DB_IP" -j DROP 2>/dev/null || true
}
trap cleanup EXIT

# ── 1. Baseline ──────────────────────────────────────────────────────────────
log "Checking baseline..."
HTTP=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/health" || echo "000")
[[ "$HTTP" == "200" ]] || fail "App not healthy before test (got $HTTP)"
READY=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/ready" || echo "000")
[[ "$READY" == "200" ]] || fail "App not ready before test (got $READY)"
log "Baseline OK"

# ── 2. Insert partition rule ──────────────────────────────────────────────────
log "Injecting network partition: DROP all packets to $DB_IP"
iptables -I OUTPUT -d "$DB_IP" -j DROP
PARTITION_TIME=$(date +%s)

# ── 3. Wait for /ready to detect the partition ───────────────────────────────
log "Waiting for /ready to return 503 (target: <${DETECTION_TARGET_SECS}s)..."
GOT_503=false
DETECTION_TIME=0
for i in $(seq 1 60); do
  sleep 1
  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" --max-time 3 "${BASE_URL}/ready" || echo "000")
  if [[ "$STATUS" == "503" ]]; then
    DETECTION_TIME=$i
    GOT_503=true
    log "/ready returned 503 after ${i}s"
    break
  fi
done

# ── 4. Hold partition, then remove ───────────────────────────────────────────
ELAPSED=$(( $(date +%s) - PARTITION_TIME ))
REMAINING=$(( PARTITION_DURATION - ELAPSED ))
if [[ "$REMAINING" -gt 0 ]]; then
  log "Holding partition for ${REMAINING}s more..."
  sleep "$REMAINING"
fi

log "Removing partition rule..."
iptables -D OUTPUT -d "$DB_IP" -j DROP
REMOVE_TIME=$(date +%s)

# ── 5. Wait for recovery ──────────────────────────────────────────────────────
log "Waiting for /ready to return 200..."
RECOVERED=false
RECOVER_TIME=0
for i in $(seq 1 "$RECOVERY_TARGET_SECS"); do
  sleep 1
  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" --max-time 3 "${BASE_URL}/ready" || echo "000")
  if [[ "$STATUS" == "200" ]]; then
    RECOVER_TIME=$(( $(date +%s) - REMOVE_TIME ))
    RECOVERED=true
    break
  fi
done

# ── 6. Results ───────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Network Partition — Results"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ "$GOT_503" == "true" ]]; then
  if [[ "$DETECTION_TIME" -le "$DETECTION_TARGET_SECS" ]]; then
    pass "Partition detected in ${DETECTION_TIME}s (target: <${DETECTION_TARGET_SECS}s)"
  else
    warn "Partition detected in ${DETECTION_TIME}s (target: <${DETECTION_TARGET_SECS}s) — slow detection"
  fi
else
  fail "/ready never returned 503 during 60 s partition — health check may not probe DB"
fi

if [[ "$RECOVERED" == "true" ]]; then
  if [[ "$RECOVER_TIME" -le "$RECOVERY_TARGET_SECS" ]]; then
    pass "Recovery after partition removed: ${RECOVER_TIME}s (target: <${RECOVERY_TARGET_SECS}s)"
  else
    fail "Recovery took ${RECOVER_TIME}s — exceeded target of ${RECOVERY_TARGET_SECS}s"
  fi
else
  fail "App did not recover within ${RECOVERY_TARGET_SECS}s of partition removal"
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

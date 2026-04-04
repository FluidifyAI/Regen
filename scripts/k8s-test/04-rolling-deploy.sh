#!/usr/bin/env bash
# 04-rolling-deploy.sh — simulate a rolling deploy and verify no downtime
# Starts a background poller hitting /health, then does helm upgrade, checks for failures.
set -euo pipefail

RELEASE_NAME="${HELM_RELEASE:-regen-test}"
NAMESPACE="${K8S_NAMESPACE:-regen-test}"
LOCAL_PORT="${LOCAL_PORT:-18081}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART_DIR="${REPO_ROOT}/deploy/helm/fluidify-regen"
VALUES_FILE="${CHART_DIR}/values.test.yaml"
POLL_INTERVAL=1
ERRORS=0
TOTAL=0
POLL_PID=""

echo "==> [04] Rolling deploy test"

# Port-forward to current deployment
POD=$(kubectl get pod -n "${NAMESPACE}" \
  -l "app.kubernetes.io/name=fluidify-regen" \
  -o jsonpath='{.items[0].metadata.name}')

kubectl port-forward -n "${NAMESPACE}" "pod/${POD}" "${LOCAL_PORT}:8080" &
PF_PID=$!
trap "kill ${PF_PID} 2>/dev/null || true; [[ -n \"${POLL_PID}\" ]] && kill ${POLL_PID} 2>/dev/null || true" EXIT
sleep 2

# Background poller — counts 5xx responses during upgrade
poll_health() {
  while true; do
    code=$(curl -so /dev/null -w "%{http_code}" --max-time 2 \
      "http://localhost:${LOCAL_PORT}/health" 2>/dev/null || echo "000")
    TOTAL=$((TOTAL + 1))
    if [[ "${code}" =~ ^5 ]] || [[ "${code}" == "000" ]]; then
      ERRORS=$((ERRORS + 1))
    fi
    sleep "${POLL_INTERVAL}"
  done
}
poll_health &
POLL_PID=$!

echo "    Running helm upgrade (same image, simulates config change)..."
helm upgrade "${RELEASE_NAME}" "${CHART_DIR}" \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_FILE}" \
  --set image.repository="fluidify-regen" \
  --set image.tag="test" \
  --set image.pullPolicy="Never" \
  --wait \
  --timeout 3m \
  --atomic

# Give poller a moment to sample post-upgrade
sleep 3
kill "${POLL_PID}" 2>/dev/null || true

echo ""
echo "    Polled ${TOTAL} requests during upgrade"
echo "    Errors (5xx / timeout): ${ERRORS}"

if [[ "${ERRORS}" -gt 0 ]]; then
  echo "    FAIL  ${ERRORS} error(s) during rolling deploy"
  exit 1
else
  echo "    PASS  Zero errors during rolling deploy"
fi

echo ""
echo "==> [04] Rolling deploy test passed"

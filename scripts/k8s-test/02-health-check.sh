#!/usr/bin/env bash
# 02-health-check.sh — verify /health and /ready endpoints via port-forward
set -euo pipefail

RELEASE_NAME="${HELM_RELEASE:-regen-test}"
NAMESPACE="${K8S_NAMESPACE:-regen-test}"
LOCAL_PORT="${LOCAL_PORT:-18080}"
PASS=0
FAIL=0

echo "==> [02] Health check"

# Find the app pod
POD=$(kubectl get pod -n "${NAMESPACE}" \
  -l "app.kubernetes.io/name=fluidify-regen" \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [[ -z "${POD}" ]]; then
  echo "    ERROR: no app pod found in namespace ${NAMESPACE}"
  kubectl get pods -n "${NAMESPACE}"
  exit 1
fi

echo "    Pod: ${POD}"

# Start port-forward in background
kubectl port-forward -n "${NAMESPACE}" "pod/${POD}" "${LOCAL_PORT}:8080" &
PF_PID=$!
trap "kill ${PF_PID} 2>/dev/null || true" EXIT
sleep 2

check() {
  local path="$1"
  local expected_field="$2"
  local expected_value="$3"

  response=$(curl -sf --max-time 5 "http://localhost:${LOCAL_PORT}${path}" 2>/dev/null || true)
  if [[ -z "${response}" ]]; then
    echo "    FAIL  ${path} — no response"
    FAIL=$((FAIL + 1))
    return
  fi

  actual=$(echo "${response}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('${expected_field}',''))" 2>/dev/null || true)
  if [[ "${actual}" == "${expected_value}" ]]; then
    echo "    PASS  ${path} — ${expected_field}=${actual}"
    PASS=$((PASS + 1))
  else
    echo "    FAIL  ${path} — expected ${expected_field}=${expected_value}, got '${actual}'"
    echo "          response: ${response}"
    FAIL=$((FAIL + 1))
  fi
}

check "/health" "status" "ok"
check "/ready"  "status" "ready"

echo ""
echo "==> [02] Results: ${PASS} passed, ${FAIL} failed"
[[ "${FAIL}" -eq 0 ]] || exit 1

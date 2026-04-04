#!/usr/bin/env bash
# 05-security-context.sh — verify pod security constraints are applied correctly
set -euo pipefail

NAMESPACE="${K8S_NAMESPACE:-regen-test}"
PASS=0
FAIL=0

echo "==> [05] Security context checks"

POD=$(kubectl get pod -n "${NAMESPACE}" \
  -l "app.kubernetes.io/name=fluidify-regen" \
  -o jsonpath='{.items[0].metadata.name}')

echo "    Pod: ${POD}"

check_field() {
  local label="$1"
  local jsonpath="$2"
  local expected="$3"

  actual=$(kubectl get pod "${POD}" -n "${NAMESPACE}" \
    -o jsonpath="${jsonpath}" 2>/dev/null || true)

  if [[ "${actual}" == "${expected}" ]]; then
    echo "    PASS  ${label} = ${actual}"
    PASS=$((PASS + 1))
  else
    echo "    FAIL  ${label}: expected '${expected}', got '${actual}'"
    FAIL=$((FAIL + 1))
  fi
}

# Pod-level security context
check_field "runAsNonRoot"           "{.spec.securityContext.runAsNonRoot}"           "true"
check_field "runAsUser"              "{.spec.securityContext.runAsUser}"              "1001"

# Container-level security context (first container)
check_field "allowPrivilegeEscalation" "{.spec.containers[0].securityContext.allowPrivilegeEscalation}" "false"
check_field "readOnlyRootFilesystem"   "{.spec.containers[0].securityContext.readOnlyRootFilesystem}"   "true"

# Verify pod is actually running (not crashlooping due to readonly fs)
PHASE=$(kubectl get pod "${POD}" -n "${NAMESPACE}" -o jsonpath='{.status.phase}')
if [[ "${PHASE}" == "Running" ]]; then
  echo "    PASS  Pod is Running (readOnlyRootFilesystem does not break startup)"
  PASS=$((PASS + 1))
else
  echo "    FAIL  Pod phase is '${PHASE}' — may be crashing due to security context"
  kubectl describe pod "${POD}" -n "${NAMESPACE}" | tail -20
  FAIL=$((FAIL + 1))
fi

echo ""
echo "==> [05] Results: ${PASS} passed, ${FAIL} failed"
[[ "${FAIL}" -eq 0 ]] || exit 1

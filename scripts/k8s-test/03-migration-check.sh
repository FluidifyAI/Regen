#!/usr/bin/env bash
# 03-migration-check.sh — verify the DB migration job completed successfully
set -euo pipefail

NAMESPACE="${K8S_NAMESPACE:-regen-test}"
PASS=0
FAIL=0

echo "==> [03] Migration job check"

# Find migration job(s)
JOBS=$(kubectl get jobs -n "${NAMESPACE}" \
  -l "app.kubernetes.io/component=migration" \
  -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true)

if [[ -z "${JOBS}" ]]; then
  # Fall back: look for any job with 'migrate' in the name
  JOBS=$(kubectl get jobs -n "${NAMESPACE}" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null \
    | grep -i migrat || true)
fi

if [[ -z "${JOBS}" ]]; then
  echo "    WARN  No migration job found — skipping (may be run as init container)"
  exit 0
fi

for JOB in ${JOBS}; do
  STATUS=$(kubectl get job "${JOB}" -n "${NAMESPACE}" \
    -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' 2>/dev/null || true)

  FAILED=$(kubectl get job "${JOB}" -n "${NAMESPACE}" \
    -o jsonpath='{.status.failed}' 2>/dev/null || echo "0")

  if [[ "${STATUS}" == "True" ]]; then
    echo "    PASS  job/${JOB} — Complete"
    PASS=$((PASS + 1))
  elif [[ "${FAILED}" -gt 0 ]]; then
    echo "    FAIL  job/${JOB} — failed (${FAILED} attempts)"
    echo "    Logs:"
    kubectl logs -n "${NAMESPACE}" "job/${JOB}" --tail=20 || true
    FAIL=$((FAIL + 1))
  else
    echo "    WARN  job/${JOB} — status unclear (may still be running)"
    kubectl get job "${JOB}" -n "${NAMESPACE}" || true
  fi
done

echo ""
echo "==> [03] Results: ${PASS} passed, ${FAIL} failed"
[[ "${FAIL}" -eq 0 ]] || exit 1

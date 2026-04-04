#!/usr/bin/env bash
# run-all.sh — run the full k8s test suite end-to-end
# Usage: bash scripts/k8s-test/run-all.sh
# Env vars:
#   K3D_CLUSTER_NAME  (default: regen-test)
#   IMAGE_TAG         (default: fluidify-regen:test)
#   HELM_RELEASE      (default: regen-test)
#   K8S_NAMESPACE     (default: regen-test)
#   SKIP_TEARDOWN     set to 1 to leave cluster running after tests
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PASS=0
FAIL=0
FAILED_STEPS=()

export K3D_CLUSTER_NAME="${K3D_CLUSTER_NAME:-regen-test}"
export IMAGE_TAG="${IMAGE_TAG:-fluidify-regen:test}"
export HELM_RELEASE="${HELM_RELEASE:-regen-test}"
export K8S_NAMESPACE="${K8S_NAMESPACE:-regen-test}"

run_step() {
  local script="$1"
  local name="$2"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  if bash "${DIR}/${script}"; then
    echo "  ✓ ${name}"
    PASS=$((PASS + 1))
  else
    echo "  ✗ ${name} — FAILED"
    FAIL=$((FAIL + 1))
    FAILED_STEPS+=("${name}")
  fi
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Fluidify Regen — Kubernetes Test Suite"
echo "  Cluster:   ${K3D_CLUSTER_NAME}"
echo "  Namespace: ${K8S_NAMESPACE}"
echo "  Image:     ${IMAGE_TAG}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Cluster setup is a hard dependency — stop immediately if it fails
if ! bash "${DIR}/00-setup-cluster.sh"; then
  echo ""
  echo "  ✗ Cluster setup + image import — FAILED (aborting suite)"
  FAIL=$((FAIL + 1))
  FAILED_STEPS+=("Cluster setup + image import")
  # Teardown whatever partial state exists
  bash "${DIR}/99-teardown.sh" 2>/dev/null || true
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Results: 0 passed, 1 failed"
  echo "  Cluster setup failed — cannot continue"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  exit 1
fi
echo "  ✓ Cluster setup + image import"
PASS=$((PASS + 1))

run_step "01-install-chart.sh"   "Helm chart install"
run_step "02-health-check.sh"    "Health and readiness probes"
run_step "03-migration-check.sh" "Migration job"
run_step "05-security-context.sh" "Pod security context"
run_step "04-rolling-deploy.sh"  "Zero-downtime rolling deploy"

# Teardown (unless skipped for debugging)
if [[ "${SKIP_TEARDOWN:-0}" != "1" ]]; then
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  bash "${DIR}/99-teardown.sh"
else
  echo ""
  echo "SKIP_TEARDOWN=1 — cluster left running at ${K3D_CLUSTER_NAME}"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Results: ${PASS} passed, ${FAIL} failed"
if [[ "${FAIL}" -gt 0 ]]; then
  echo "  Failed steps:"
  for s in "${FAILED_STEPS[@]}"; do
    echo "    - ${s}"
  done
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  exit 1
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

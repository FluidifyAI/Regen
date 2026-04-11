#!/usr/bin/env bash
# 01-install-chart.sh — install the Helm chart and wait for pods to be ready
set -euo pipefail

CLUSTER_NAME="${K3D_CLUSTER_NAME:-regen-test}"
RELEASE_NAME="${HELM_RELEASE:-regen-test}"
NAMESPACE="${K8S_NAMESPACE:-regen-test}"
IMAGE_TAG="${IMAGE_TAG:-fluidify-regen:test}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART_DIR="${REPO_ROOT}/deploy/helm/fluidify-regen"
VALUES_FILE="${CHART_DIR}/values.test.yaml"

echo "==> [01] Installing Helm chart"
echo "    Release:   ${RELEASE_NAME}"
echo "    Namespace: ${NAMESPACE}"
echo "    Chart:     ${CHART_DIR}"

# Update Bitnami subchart dependencies
echo "    Updating chart dependencies..."
helm dependency update "${CHART_DIR}"

# Pre-pull subchart images into k3d so helm install --wait doesn't block on pulls.
# Must use --platform linux/amd64 — k3d's containerd cannot import multi-arch
# manifest lists; it needs a single-arch image tarball.
echo "    Pre-loading subchart images into k3d..."
SUBCHART_IMAGES=$(helm template "${RELEASE_NAME}" "${CHART_DIR}" \
  --values "${VALUES_FILE}" \
  --set image.repository="fluidify-regen" \
  --set image.tag="test" \
  2>/dev/null \
  | grep -E '^\s+image:' \
  | awk '{print $2}' \
  | tr -d '"' \
  | sort -u \
  | grep -v "^fluidify-regen")
for img in ${SUBCHART_IMAGES}; do
  echo "    Pulling: ${img}"
  docker pull --platform linux/amd64 "${img}" --quiet
  docker save "${img}" | k3d image import --cluster "${CLUSTER_NAME}" -
done

# Create namespace (idempotent)
kubectl create namespace "${NAMESPACE}" 2>/dev/null || true

# Clear any stuck Helm release state (can happen if a previous install timed out)
if helm status "${RELEASE_NAME}" --namespace "${NAMESPACE}" 2>/dev/null | grep -q "pending"; then
  echo "    Clearing stuck Helm release state..."
  helm uninstall "${RELEASE_NAME}" --namespace "${NAMESPACE}" --no-hooks 2>/dev/null || true
  sleep 2
fi

# Install or upgrade
helm upgrade --install "${RELEASE_NAME}" "${CHART_DIR}" \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_FILE}" \
  --set image.repository="fluidify-regen" \
  --set image.tag="test" \
  --set image.pullPolicy="Never" \
  --wait \
  --timeout 12m \
  --atomic

echo ""
echo "==> [01] Pods:"
kubectl get pods -n "${NAMESPACE}"

echo ""
echo "==> [01] Chart installed successfully"

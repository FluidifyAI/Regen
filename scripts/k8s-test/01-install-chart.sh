#!/usr/bin/env bash
# 01-install-chart.sh — install the Helm chart and wait for pods to be ready
set -euo pipefail

CLUSTER_NAME="${K3D_CLUSTER_NAME:-regen-test}"
RELEASE_NAME="${HELM_RELEASE:-regen-test}"
NAMESPACE="${K8S_NAMESPACE:-regen-test}"
IMAGE_TAG="${IMAGE_TAG:-fluidify-regen:test}"
REGISTRY_NAME="regen-registry"
REGISTRY_PORT="5001"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART_DIR="${REPO_ROOT}/deploy/helm/fluidify-regen"
VALUES_FILE="${CHART_DIR}/values.test.yaml"

echo "==> [01] Installing Helm chart"
echo "    Release:   ${RELEASE_NAME}"
echo "    Namespace: ${NAMESPACE}"
echo "    Chart:     ${CHART_DIR}"

# Chart tarballs (postgresql, redis) are checked into charts/ — no dependency
# fetch needed. Helm reads them directly from the charts/ directory.

# Mirror Bitnami subchart images into the local registry so k3d's containerd
# pulls from there instead of Docker Hub. docker pull on the runner host is fast
# and uses the Docker daemon cache between runs. We avoid the OCI+zstd import
# failures that occur with k3d image import for Bitnami images.
echo "    Mirroring subchart images to local registry..."
BITNAMI_IMAGES=(
  "registry-1.docker.io/bitnami/postgresql:latest"
  "registry-1.docker.io/bitnami/redis:latest"
  "registry-1.docker.io/bitnami/os-shell:latest"
)
for img in "${BITNAMI_IMAGES[@]}"; do
  short="${img#registry-1.docker.io/}"   # bitnami/postgresql:latest
  local_ref="k3d-${REGISTRY_NAME}:${REGISTRY_PORT}/${short}"
  echo "    Pulling: ${img}"
  docker pull --platform linux/amd64 "${img}" --quiet
  docker tag "${img}" "${local_ref}"
  docker push "${local_ref}" --quiet
  echo "    Mirrored → ${local_ref}"
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

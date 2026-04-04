#!/usr/bin/env bash
# 00-setup-cluster.sh — create a k3d test cluster and load the Docker image
set -euo pipefail

CLUSTER_NAME="${K3D_CLUSTER_NAME:-regen-test}"
IMAGE_TAG="${IMAGE_TAG:-fluidify-regen:test}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "==> [00] Setting up k3d cluster: ${CLUSTER_NAME}"

# Delete existing cluster if present (idempotent)
if k3d cluster list | grep -q "^${CLUSTER_NAME}"; then
  echo "    Deleting existing cluster..."
  k3d cluster delete "${CLUSTER_NAME}"
fi

echo "    Creating cluster..."
k3d cluster create "${CLUSTER_NAME}" \
  --wait \
  --timeout 120s

echo "    Cluster ready. Nodes:"
kubectl get nodes

echo ""
echo "==> [00] Building Docker image: ${IMAGE_TAG}"
docker build -t "${IMAGE_TAG}" "${REPO_ROOT}" --quiet

echo "==> [00] Importing image into k3d..."
k3d image import "${IMAGE_TAG}" --cluster "${CLUSTER_NAME}"

echo "==> [00] Setup complete"

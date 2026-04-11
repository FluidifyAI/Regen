#!/usr/bin/env bash
# 00-setup-cluster.sh — create a k3d test cluster and load the Docker image
set -euo pipefail

CLUSTER_NAME="${K3D_CLUSTER_NAME:-regen-test}"
REGISTRY_NAME="regen-registry"
REGISTRY_PORT="5001"
IMAGE_TAG="${IMAGE_TAG:-fluidify-regen:test}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "==> [00] Setting up k3d cluster: ${CLUSTER_NAME}"

# Create a local registry if it doesn't already exist.
# The registry is intentionally kept alive between runs — it acts as a
# persistent pull-through cache for Bitnami images on the self-hosted runner.
if ! k3d registry list 2>/dev/null | grep -q "${REGISTRY_NAME}"; then
  echo "    Creating local registry k3d-${REGISTRY_NAME}:${REGISTRY_PORT}..."
  k3d registry create "${REGISTRY_NAME}" --port "${REGISTRY_PORT}"
else
  echo "    Registry k3d-${REGISTRY_NAME}:${REGISTRY_PORT} already exists — reusing"
fi

# Write a containerd registry mirror config so k3d's containerd redirects
# registry-1.docker.io pulls to the local registry first. This avoids Docker
# Hub rate limits and the OCI+zstd import failures we see with k3d image import.
cat > /tmp/k3d-registries.yaml << EOF
mirrors:
  "registry-1.docker.io":
    endpoint:
      - "http://k3d-${REGISTRY_NAME}:${REGISTRY_PORT}"
EOF

# Delete existing cluster if present (idempotent)
if k3d cluster list | grep -q "^${CLUSTER_NAME}"; then
  echo "    Deleting existing cluster..."
  k3d cluster delete "${CLUSTER_NAME}"
fi

echo "    Creating cluster..."
k3d cluster create "${CLUSTER_NAME}" \
  --registry-use "k3d-${REGISTRY_NAME}:${REGISTRY_PORT}" \
  --registry-config /tmp/k3d-registries.yaml \
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

#!/usr/bin/env bash
# 99-teardown.sh — delete the k3d test cluster
set -euo pipefail

CLUSTER_NAME="${K3D_CLUSTER_NAME:-regen-test}"

echo "==> [99] Tearing down k3d cluster: ${CLUSTER_NAME}"

if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
  k3d cluster delete "${CLUSTER_NAME}"
  echo "    Cluster deleted"
else
  echo "    Cluster not found — nothing to do"
fi

echo "==> [99] Teardown complete"

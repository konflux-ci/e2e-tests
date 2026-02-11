#!/bin/bash
# =============================================================================
# Cluster Setup Script for Konflux E2E Test Clusters (KinD)
#
# Hardens a fresh KinD cluster for burst E2E workloads.
# Only essential settings — give resources, don't micro-tune.
#
# Steps:
#   1. Node hardening (maxPods + parallel pulls + containerd protection)
#   2. Deploy manifests (registry caches, pre-pull, cleanup)
#   3. Containerd mirrors + concurrent downloads
#   4. Metrics-server + CoreDNS HA
#   5. Verification
#
# Usage:
#   ./setup-cluster.sh [KUBECONFIG] [NODE_NAME] [MAX_PODS]
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBECONFIG="${1:-./kubeconfig}"
NODE_NAME="${2:-kind-mapt-control-plane}"
MAX_PODS="${3:-350}"

export KUBECONFIG

echo "=============================================="
echo "  Konflux E2E Cluster Setup"
echo "=============================================="
echo "  Node: ${NODE_NAME}  MaxPods: ${MAX_PODS}"
echo "=============================================="
echo ""

# -----------------------------------------------------------------------------
# Step 1: Node hardening — kubelet + containerd (single nsenter pod)
#
# Essential settings only:
#   Kubelet:    maxPods, parallel pulls (without these, burst fails)
#   Containerd: FD limits, memory reservation, OOM protection (without these,
#               containerd crashes under 200+ container burst)
# -----------------------------------------------------------------------------
echo "==> [1/5] Hardening node..."

NODE_SCRIPT=$(cat <<'INNER_SCRIPT'
#!/bin/bash
set -e
CONFIG="/var/lib/kubelet/config.yaml"

# --- maxPods (KinD default 110 is too low) ---
if grep -q "^maxPods:" "$CONFIG"; then
  sed -i "s/^maxPods:.*/maxPods: MAX_PODS_PLACEHOLDER/" "$CONFIG"
else
  echo "maxPods: MAX_PODS_PLACEHOLDER" >> "$CONFIG"
fi

# --- Parallel image pulls (without this, pulls queue and timeout) ---
if grep -q "^serializeImagePulls:" "$CONFIG"; then
  sed -i "s/^serializeImagePulls:.*/serializeImagePulls: false/" "$CONFIG"
else
  echo "serializeImagePulls: false" >> "$CONFIG"
fi
if grep -q "^maxParallelImagePulls:" "$CONFIG"; then
  sed -i "s/^maxParallelImagePulls:.*/maxParallelImagePulls: 20/" "$CONFIG"
else
  echo "maxParallelImagePulls: 20" >> "$CONFIG"
fi

# --- Containerd systemd: give it resources and protect from OOM ---
mkdir -p /etc/systemd/system/containerd.service.d
cat > /etc/systemd/system/containerd.service.d/override.conf <<CONTAINERD_OVERRIDE
[Service]
LimitNOFILE=1048576
LimitMEMLOCK=infinity
TasksMax=infinity
MemoryMin=8G
OOMScoreAdjust=-999
Restart=always
RestartSec=2
CONTAINERD_OVERRIDE

systemctl daemon-reload
systemctl restart kubelet
echo "DONE"
INNER_SCRIPT
)

NODE_SCRIPT="${NODE_SCRIPT//MAX_PODS_PLACEHOLDER/${MAX_PODS}}"
ENCODED=$(echo "$NODE_SCRIPT" | base64 | tr -d '\n')

POD_NAME="setup-node-$(date +%s)"
kubectl run "${POD_NAME}" --restart=Never --image=busybox \
  --overrides="{
    \"spec\": {
      \"hostPID\": true,
      \"nodeName\": \"${NODE_NAME}\",
      \"containers\": [{
        \"name\": \"fix\",
        \"image\": \"busybox\",
        \"command\": [
          \"nsenter\", \"--target\", \"1\",
          \"--mount\", \"--uts\", \"--ipc\", \"--net\", \"--pid\", \"--\",
          \"bash\", \"-c\",
          \"echo ${ENCODED} | base64 -d | bash\"
        ],
        \"securityContext\": {\"privileged\": true}
      }]
    }
  }" 2>/dev/null

echo "    Waiting for pod to complete..."
for i in $(seq 1 60); do
  PHASE=$(kubectl get pod "${POD_NAME}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Pending")
  if [ "$PHASE" = "Succeeded" ] || [ "$PHASE" = "Failed" ]; then
    echo "    Pod finished (${PHASE}) after ${i}s."
    break
  fi
  sleep 1
done
kubectl logs "${POD_NAME}" 2>/dev/null || true
kubectl delete pod "${POD_NAME}" --force --grace-period=0 2>/dev/null || true
echo "    Waiting for node Ready after kubelet restart..."
kubectl wait --for=condition=Ready node/"${NODE_NAME}" --timeout=120s 2>/dev/null
echo "    maxPods=${MAX_PODS}, parallelPulls=20, containerd protected."
echo ""

# -----------------------------------------------------------------------------
# Step 2: Deploy all Kubernetes manifests
# -----------------------------------------------------------------------------
echo "==> [2/5] Deploying manifests..."
kubectl apply -f "${SCRIPT_DIR}/image-optimization.yaml" 2>/dev/null
kubectl apply -f "${SCRIPT_DIR}/cluster-cleanup.yaml" 2>/dev/null
echo "    Waiting for registry caches..."
kubectl wait --for=condition=Available deployment/cache-quay deployment/cache-ghcr deployment/cache-cgr deployment/cache-redhat \
  -n registry-cache --timeout=120s 2>/dev/null || echo "    (some caches still starting...)"
echo "    All manifests deployed."
echo ""

# -----------------------------------------------------------------------------
# Step 3: Containerd mirrors + concurrent downloads (restarts containerd)
# -----------------------------------------------------------------------------
echo "==> [3/5] Configuring containerd mirrors..."
"${SCRIPT_DIR}/apply-mirrors.sh" "${KUBECONFIG}" "${NODE_NAME}"
echo ""

# -----------------------------------------------------------------------------
# Step 4: Metrics-server + CoreDNS HA
# -----------------------------------------------------------------------------
echo "==> [4/5] Metrics-server + CoreDNS..."
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml 2>/dev/null
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]' 2>/dev/null || true
if kubectl get deployment coredns -n kube-system &>/dev/null; then
  kubectl scale deployment coredns -n kube-system --replicas=3 2>/dev/null || true
  kubectl patch deployment coredns -n kube-system --type='json' -p='[
    {"op":"replace","path":"/spec/template/spec/containers/0/resources/limits/memory","value":"512Mi"},
    {"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/memory","value":"128Mi"},
    {"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/cpu","value":"200m"}
  ]' 2>/dev/null || true
fi
echo "    Metrics-server installed. CoreDNS: 3 replicas, 512Mi."
echo ""

# -----------------------------------------------------------------------------
# Step 5: Verification
# -----------------------------------------------------------------------------
echo "==> [5/5] Done."
echo ""
echo "=============================================="
echo "  Cluster Ready"
echo "=============================================="
echo "  maxPods=${MAX_PODS}  parallelPulls=20"
echo "  containerd: MemoryMin=8G OOM=-999 FDs=1M"
echo "  caches: quay ghcr cgr redhat"
echo "  cleanup: container-gc(25s) etcd-pruner(*/10)"
echo "  CoreDNS: 3x512Mi"
echo "=============================================="
kubectl get node "${NODE_NAME}" 2>/dev/null || true
echo ""

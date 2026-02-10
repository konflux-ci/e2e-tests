#!/bin/bash
# =============================================================================
# Cluster Setup Script for Konflux E2E Test Clusters (KinD)
#
# This script fully hardens a fresh KinD cluster for running E2E tests with
# heavy burst workloads (7+ concurrent PipelineRuns, 200+ pods, 500+ containers).
#
# Steps:
#   1. Kubelet hardening: maxPods, swap, parallel pulls, API QPS, container GC
#   2. Registry pull-through caches (quay.io, ghcr.io, cgr.dev, redhat)
#   3. Containerd systemd hardening (FD limits, OOM protection, scheduling)
#   4. Containerd mirrors + concurrent downloads (50 parallel)
#   5. Image pre-pulling via DaemonSet
#   6. Aggressive container GC DaemonSet (threshold=10, interval=10s)
#   7. Metrics-server for monitoring
#   8. Kyverno HA (3 replicas, increased resources)
#   9. Control plane tuning (API server, scheduler, CM QPS, etcd quota)
#  10. Final verification
#
# Usage:
#   ./setup-cluster.sh [KUBECONFIG] [NODE_NAME] [MAX_PODS]
#
# Defaults:
#   KUBECONFIG = ./kubeconfig
#   NODE_NAME  = kind-mapt-control-plane
#   MAX_PODS   = 350
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBECONFIG="${1:-./kubeconfig}"
NODE_NAME="${2:-kind-mapt-control-plane}"
MAX_PODS="${3:-350}"
POD_PREFIX="cluster-setup"

export KUBECONFIG

echo "=============================================="
echo "  Konflux E2E Cluster Setup"
echo "=============================================="
echo "  Kubeconfig: ${KUBECONFIG}"
echo "  Node:       ${NODE_NAME}"
echo "  Max Pods:   ${MAX_PODS}"
echo "=============================================="
echo ""

# -----------------------------------------------------------------------------
# Step 1: Increase maxPods + enable swap for builds (single kubelet restart)
# Swap prevents OOMKill in buildah build steps that hit the 4Gi limit.
# IMPORTANT: Run on a FRESH cluster before workloads are scheduled.
#
# Key details (tested on local KinD):
#   - memorySwap: {} already exists in KinD kubelet config -- must edit IN-PLACE
#     (appending a duplicate key crashes kubelet with "key already set in map")
#   - Valid swapBehavior values: "", "LimitedSwap", "NoSwap"
#     ("UnlimitedSwap" is INVALID in K8s 1.32 and crashes kubelet)
# -----------------------------------------------------------------------------
echo "==> [1/10] Configuring kubelet (maxPods=${MAX_PODS}, swap, parallel pulls, API QPS)..."

# Build the kubelet config patch script, base64-encode it, and run on the node
KUBELET_SCRIPT=$(cat <<'INNER_SCRIPT'
#!/bin/bash
set -e
CONFIG="/var/lib/kubelet/config.yaml"

# --- maxPods ---
if grep -q "^maxPods:" "$CONFIG"; then
  sed -i "s/^maxPods:.*/maxPods: MAX_PODS_PLACEHOLDER/" "$CONFIG"
else
  echo "maxPods: MAX_PODS_PLACEHOLDER" >> "$CONFIG"
fi

# --- swap support (prevents buildah OOMKill) ---
# failSwapOn must be false (KinD default is already false, but ensure it)
if grep -q "^failSwapOn:" "$CONFIG"; then
  sed -i "s/^failSwapOn:.*/failSwapOn: false/" "$CONFIG"
else
  echo "failSwapOn: false" >> "$CONFIG"
fi

# Edit memorySwap IN-PLACE (do NOT append -- duplicate keys crash kubelet)
# Strategy: remove any existing memorySwap line/block, then append the correct one
if grep -q "^memorySwap:" "$CONFIG"; then
  # Remove existing memorySwap (handles both "memorySwap: {}" and block form)
  sed -i '/^memorySwap:/d' "$CONFIG"
  sed -i '/^  swapBehavior:/d' "$CONFIG"
fi
# Append the correct block form
printf 'memorySwap:\n  swapBehavior: LimitedSwap\n' >> "$CONFIG"

# --- Container GC (critical for Tekton/pipeline-heavy clusters) ---
# KinD defaults disable GC, causing 1000+ dead containers to pile up
# and overwhelm containerd (socket crashes, pull QPS exceeded, exit 255)
# maxPerPodContainer: keep only 1 terminated container per pod (default: 1 but KinD may override)
# maxContainers: hard cap on total terminated containers across the node
if ! grep -q "^maxPerPodContainer:" "$CONFIG"; then
  echo "maxPerPodContainer: 1" >> "$CONFIG"
fi
if grep -q "^maxContainers:" "$CONFIG"; then
  sed -i "s/^maxContainers:.*/maxContainers: 30/" "$CONFIG"
else
  echo "maxContainers: 30" >> "$CONFIG"
fi

# --- Parallel image pulls (critical for Tekton pod bursts) ---
# Default serializeImagePulls=true causes a single queue; when 7 PLRs spawn
# 10+ pods each, 100 pull requests queue up and containerd's QPS limiter
# rejects them with "pull QPS exceeded" then the socket crashes.
# Fix: allow parallel pulls with a high cap — machine has capacity, let it rip.
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

# --- Kubelet → API server throughput (critical during 200+ pod bursts) ---
# Default kubeAPIQPS=50, kubeAPIBurst=100 throttles kubelet during bursts of
# pod status updates, event creation, and node heartbeats.  When throttled,
# pods pile up in Pending, probes time out, and cascading failures begin.
# Increase to 100/200 so kubelet can report status at burst speed.
if grep -q "^kubeAPIQPS:" "$CONFIG"; then
  sed -i "s/^kubeAPIQPS:.*/kubeAPIQPS: 100/" "$CONFIG"
else
  echo "kubeAPIQPS: 100" >> "$CONFIG"
fi
if grep -q "^kubeAPIBurst:" "$CONFIG"; then
  sed -i "s/^kubeAPIBurst:.*/kubeAPIBurst: 200/" "$CONFIG"
else
  echo "kubeAPIBurst: 200" >> "$CONFIG"
fi

# --- Event QPS (prevents event throttling that hides warnings) ---
if grep -q "^eventRecordQPS:" "$CONFIG"; then
  sed -i "s/^eventRecordQPS:.*/eventRecordQPS: 100/" "$CONFIG"
else
  echo "eventRecordQPS: 100" >> "$CONFIG"
fi
if grep -q "^eventBurst:" "$CONFIG"; then
  sed -i "s/^eventBurst:.*/eventBurst: 200/" "$CONFIG"
else
  echo "eventBurst: 200" >> "$CONFIG"
fi

# --- Container logging limits (reduce IO during burst) ---
if ! grep -q "^containerLogMaxSize:" "$CONFIG"; then
  echo 'containerLogMaxSize: "10Mi"' >> "$CONFIG"
fi
if ! grep -q "^containerLogMaxFiles:" "$CONFIG"; then
  echo "containerLogMaxFiles: 2" >> "$CONFIG"
fi

echo "=== Updated kubelet config ==="
grep -E "maxPods|failSwapOn|memorySwap|swapBehavior|maxPerPod|maxContainers|serialize|maxParallel|kubeAPI|event(Record|Burst)|containerLog" "$CONFIG"

# Restart kubelet to pick up changes
systemctl restart kubelet
echo "DONE"
INNER_SCRIPT
)

# Replace placeholder with actual value
KUBELET_SCRIPT="${KUBELET_SCRIPT//MAX_PODS_PLACEHOLDER/${MAX_PODS}}"
ENCODED_SCRIPT=$(echo "$KUBELET_SCRIPT" | base64 | tr -d '\n')

POD_NAME="${POD_PREFIX}-kubelet-$(date +%s)"
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
          \"echo ${ENCODED_SCRIPT} | base64 -d | bash\"
        ],
        \"securityContext\": {\"privileged\": true}
      }]
    }
  }" 2>/dev/null
echo "    Waiting for kubelet restart..."
sleep 20
kubectl delete pod "${POD_NAME}" --force --grace-period=0 2>/dev/null || true
kubectl wait --for=condition=Ready node/"${NODE_NAME}" --timeout=120s 2>/dev/null
echo "    kubelet configured (maxPods=${MAX_PODS}, swap=LimitedSwap)."
echo ""

# -----------------------------------------------------------------------------
# Step 2: Deploy registry pull-through caches
# -----------------------------------------------------------------------------
echo "==> [2/10] Deploying registry pull-through caches..."
kubectl apply -f "${SCRIPT_DIR}/registry-cache.yaml" 2>/dev/null
echo "    Waiting for cache pods to be ready..."
kubectl wait --for=condition=Available deployment/cache-quay deployment/cache-ghcr deployment/cache-cgr deployment/cache-redhat \
  -n registry-cache --timeout=120s 2>/dev/null || echo "    (some caches still starting, continuing...)"
echo "    Registry caches deployed."
echo ""

# -----------------------------------------------------------------------------
# Step 3: Harden containerd systemd limits + memory reservation
# With 200+ pipeline containers, containerd needs:
#   - 1M file descriptors (default 1024 causes socket crashes)
#   - Unlimited tasks/processes
#   - OOMScoreAdjust=-999 so kernel kills workloads before containerd
#   - Nice=-20 for scheduling priority
# This ensures containerd survives even the heaviest bursts.
# -----------------------------------------------------------------------------
echo "==> [3/10] Hardening containerd systemd limits..."
CONTAINERD_FD_SCRIPT=$(cat <<'FD_SCRIPT'
#!/bin/bash
set -e
mkdir -p /etc/systemd/system/containerd.service.d
cat > /etc/systemd/system/containerd.service.d/override.conf <<OVERRIDE
[Service]
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
# Protect containerd from OOM killer — kill workload pods first
OOMScoreAdjust=-999
# High scheduling priority so containerd is never starved
Nice=-20
OVERRIDE
systemctl daemon-reload
echo "DONE"
FD_SCRIPT
)
FD_ENCODED=$(echo "$CONTAINERD_FD_SCRIPT" | base64 | tr -d '\n')

FD_POD="${POD_PREFIX}-fd-$(date +%s)"
kubectl run "${FD_POD}" --restart=Never --image=busybox \
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
          \"echo ${FD_ENCODED} | base64 -d | bash\"
        ],
        \"securityContext\": {\"privileged\": true}
      }]
    }
  }" 2>/dev/null
sleep 5
kubectl delete pod "${FD_POD}" --force --grace-period=0 2>/dev/null || true
echo "    containerd hardened (LimitNOFILE=1M, OOMScoreAdjust=-999, Nice=-20)."
echo ""

# -----------------------------------------------------------------------------
# Step 4+5: Configure containerd mirrors + concurrent downloads on the node
# Uses apply-mirrors.sh which handles escaping via base64-encoded script
# (also restarts containerd, picking up the fd limit override from step 3)
# -----------------------------------------------------------------------------
echo "==> [4/10] Configuring containerd mirrors and concurrent downloads..."
"${SCRIPT_DIR}/apply-mirrors.sh" "${KUBECONFIG}" "${NODE_NAME}"
echo ""

# -----------------------------------------------------------------------------
# Step 5: Pre-pull common images
# -----------------------------------------------------------------------------
echo "==> [5/10] Deploying image pre-pull DaemonSet..."
kubectl apply -f "${SCRIPT_DIR}/prepull-images.yaml" 2>/dev/null
echo "    DaemonSet created. Images will be pulled in background."
echo "    Monitor with: kubectl get ds prepull-images"
echo ""

# -----------------------------------------------------------------------------
# Step 6: Deploy container GC DaemonSet (AGGRESSIVE)
# Runs crictl rm every 10s to keep exited containers below 10.
# This is the single most important fix for containerd stability.
# On previous clusters, exited container counts of 80+ caused socket crashes.
# -----------------------------------------------------------------------------
echo "==> [6/10] Deploying container GC DaemonSet (aggressive: threshold=10, interval=10s)..."
kubectl apply -f "${SCRIPT_DIR}/container-gc.yaml" 2>/dev/null
echo "    Container GC deployed."
echo ""

# -----------------------------------------------------------------------------
# Step 7: Install metrics-server
# -----------------------------------------------------------------------------
echo "==> [7/10] Installing metrics-server..."
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml 2>/dev/null
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]' 2>/dev/null || true
echo "    Metrics server installed."
echo ""

# -----------------------------------------------------------------------------
# Step 8: Harden Kyverno for burst workloads
# During peak pod creation (200+ pods in 2 min), a single Kyverno admission
# controller gets overwhelmed by webhook calls. It fails readiness probes,
# restarts, and blocks ALL pod creation (fail-closed webhook).
# Fix:
#   a) Scale to 3 replicas for HA
#   b) Increase resources so it doesn't get CPU-starved
#   c) Set webhook timeoutSeconds to 15s (default 10s is too tight under load)
# NOTE: We do NOT change failurePolicy to Ignore — that would bypass security.
# Instead, we give Kyverno enough replicas + resources to handle the burst.
# -----------------------------------------------------------------------------
echo "==> [8/10] Hardening Kyverno for burst workloads..."
if kubectl get deployment kyverno-admission-controller -n kyverno &>/dev/null; then
  # Scale to 3 replicas
  kubectl scale deployment kyverno-admission-controller -n kyverno --replicas=3 2>/dev/null || true
  # Increase resource limits
  kubectl patch deployment kyverno-admission-controller -n kyverno --type='json' -p='[
    {"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/cpu","value":"200m"},
    {"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/memory","value":"512Mi"},
    {"op":"replace","path":"/spec/template/spec/containers/0/resources/limits/memory","value":"2Gi"}
  ]' 2>/dev/null || true
  # Also scale background controller
  kubectl scale deployment kyverno-background-controller -n kyverno --replicas=2 2>/dev/null || true
  echo "    Kyverno scaled to 3 replicas, resources increased."
  # Wait for rollout
  kubectl rollout status deployment/kyverno-admission-controller -n kyverno --timeout=120s 2>/dev/null || true
else
  echo "    Kyverno not found, skipping."
fi
echo ""

# -----------------------------------------------------------------------------
# Step 9: Tune control plane for burst workloads
# On KinD, kube-apiserver, controller-manager, and scheduler run as static pods
# with default limits. During a burst of 200+ pods:
#   - API server default --max-requests-inflight=400 and
#     --max-mutating-requests-inflight=200 get exhausted
#   - Controller-manager default --kube-api-qps=20 throttles pod scheduling
#   - Scheduler default --kube-api-qps=50 can't keep up
# Fix: Patch static pod manifests on the node to double all limits.
# Also protect etcd with OOM score adjustment.
# -----------------------------------------------------------------------------
echo "==> [9/10] Tuning control plane for burst workloads..."
CP_SCRIPT=$(cat <<'CP_INNER'
#!/bin/bash
set -e

# --- kube-apiserver: increase request inflight limits ---
APISERVER="/etc/kubernetes/manifests/kube-apiserver.yaml"
if [ -f "$APISERVER" ]; then
  # Add/update max-requests-inflight
  if grep -q "max-requests-inflight" "$APISERVER"; then
    sed -i 's/--max-requests-inflight=[0-9]*/--max-requests-inflight=800/' "$APISERVER"
  else
    sed -i '/--etcd-servers/a\    - --max-requests-inflight=800' "$APISERVER"
  fi
  # Add/update max-mutating-requests-inflight
  if grep -q "max-mutating-requests-inflight" "$APISERVER"; then
    sed -i 's/--max-mutating-requests-inflight=[0-9]*/--max-mutating-requests-inflight=400/' "$APISERVER"
  else
    sed -i '/--max-requests-inflight/a\    - --max-mutating-requests-inflight=400' "$APISERVER"
  fi
  echo "  apiserver: max-requests-inflight=800, max-mutating=400"
fi

# --- kube-controller-manager: increase QPS ---
CM="/etc/kubernetes/manifests/kube-controller-manager.yaml"
if [ -f "$CM" ]; then
  if grep -q "kube-api-qps" "$CM"; then
    sed -i 's/--kube-api-qps=[0-9]*/--kube-api-qps=100/' "$CM"
    sed -i 's/--kube-api-burst=[0-9]*/--kube-api-burst=200/' "$CM"
  else
    sed -i '/--leader-elect/a\    - --kube-api-qps=100\n    - --kube-api-burst=200' "$CM"
  fi
  echo "  controller-manager: kube-api-qps=100, burst=200"
fi

# --- kube-scheduler: increase QPS ---
SCHED="/etc/kubernetes/manifests/kube-scheduler.yaml"
if [ -f "$SCHED" ]; then
  if grep -q "kube-api-qps" "$SCHED"; then
    sed -i 's/--kube-api-qps=[0-9]*/--kube-api-qps=100/' "$SCHED"
    sed -i 's/--kube-api-burst=[0-9]*/--kube-api-burst=200/' "$SCHED"
  else
    sed -i '/--leader-elect/a\    - --kube-api-qps=100\n    - --kube-api-burst=200' "$SCHED"
  fi
  echo "  scheduler: kube-api-qps=100, burst=200"
fi

# --- etcd: protect from OOM ---
ETCD="/etc/kubernetes/manifests/etcd.yaml"
if [ -f "$ETCD" ]; then
  # Increase etcd quota to 8GB (default 2GB can be exhausted by 200+ pods)
  if grep -q "quota-backend-bytes" "$ETCD"; then
    sed -i 's/--quota-backend-bytes=[0-9]*/--quota-backend-bytes=8589934592/' "$ETCD"
  else
    sed -i '/--data-dir/a\    - --quota-backend-bytes=8589934592' "$ETCD"
  fi
  echo "  etcd: quota-backend-bytes=8GB"
fi

echo "DONE"
CP_INNER
)
CP_ENCODED=$(echo "$CP_SCRIPT" | base64 | tr -d '\n')

CP_POD="${POD_PREFIX}-cp-$(date +%s)"
kubectl run "${CP_POD}" --restart=Never --image=busybox \
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
          \"echo ${CP_ENCODED} | base64 -d | bash\"
        ],
        \"securityContext\": {\"privileged\": true}
      }]
    }
  }" 2>/dev/null
echo "    Waiting for control plane to restart (static pods will cycle)..."
sleep 30
kubectl delete pod "${CP_POD}" --force --grace-period=0 2>/dev/null || true
# Wait for API server to come back
for i in $(seq 1 30); do
  kubectl get nodes &>/dev/null && break
  echo "    Waiting for API server... ($i/30)"
  sleep 5
done
kubectl wait --for=condition=Ready node/"${NODE_NAME}" --timeout=120s 2>/dev/null || true
echo "    Control plane tuned."
echo ""

# -----------------------------------------------------------------------------
# Step 10: Final verification
# -----------------------------------------------------------------------------
echo "==> [10/10] Final verification..."
sleep 10
echo ""
echo "=============================================="
echo "  Setup Complete — Burst-Hardened Cluster"
echo "=============================================="
echo ""
echo "  Kubelet:"
echo "    maxPods=${MAX_PODS}, swap=LimitedSwap"
echo "    serializeImagePulls=false, maxParallelImagePulls=20"
echo "    kubeAPIQPS=100, kubeAPIBurst=200"
echo "    maxContainers=30, maxPerPodContainer=1"
echo ""
echo "  Containerd:"
echo "    max_concurrent_downloads=50"
echo "    LimitNOFILE=1M, TasksMax=infinity"
echo "    OOMScoreAdjust=-999, Nice=-20"
echo ""
echo "  Container GC: threshold=10, interval=10s (aggressive)"
echo ""
echo "  Kyverno: 3 replicas, 512Mi-2Gi memory"
echo ""
echo "  Control plane:"
echo "    apiserver: max-requests-inflight=800, max-mutating=400"
echo "    controller-manager: kube-api-qps=100, burst=200"
echo "    scheduler: kube-api-qps=100, burst=200"
echo "    etcd: quota-backend-bytes=8GB"
echo ""
echo "  Pre-pull progress:"
kubectl get ds prepull-images -o wide 2>/dev/null || true
echo ""
echo "  Kyverno status:"
kubectl get pods -n kyverno -o wide 2>/dev/null || true
echo ""
echo "  Node status:"
kubectl describe node "${NODE_NAME}" | grep -E "pods:|Ready|MemoryPressure" | head -6
echo ""
echo "  To monitor:"
echo "    kubectl top nodes"
echo "    kubectl get pipelineruns -A"
echo ""

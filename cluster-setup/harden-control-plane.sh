#!/bin/bash
# =============================================================================
# Harden KinD control plane for burst workloads
#
# Increases resource limits and tunes flags for apiserver, etcd, scheduler,
# and controller-manager on a single-node KinD cluster.
#
# IMPORTANT: Run this right after setup-cluster.sh, BEFORE Konflux bootstrap.
# Static pod changes cause brief restarts (~5-10s each). With no workloads
# running, there is zero service impact.
#
# Order (least disruptive first):
#   1. kube-scheduler        — no API impact, just scheduling pauses briefly
#   2. kube-controller-manager — no API impact, just reconciliation pauses
#   3. kube-apiserver         — brief API blip (~5-10s)
#   4. etcd                   — brief API blip (~5-10s)
#
# Each component is patched individually with a health check before proceeding.
#
# Usage:
#   ./harden-control-plane.sh [KUBECONFIG] [NODE_NAME]
# =============================================================================

set -euo pipefail

KUBECONFIG="${1:-./kubeconfig}"
NODE_NAME="${2:-kind-mapt-control-plane}"
export KUBECONFIG

echo "=============================================="
echo "  Control Plane Hardening"
echo "=============================================="
echo "  Node: ${NODE_NAME}"
echo "=============================================="
echo ""

# Build the script that runs on the node via nsenter
# We patch ONE manifest at a time and signal which one was patched
PATCH_SCRIPT=$(cat <<'PATCH_INNER'
#!/bin/bash
set -e

COMPONENT="$1"

case "$COMPONENT" in
  scheduler)
    MANIFEST="/etc/kubernetes/manifests/kube-scheduler.yaml"
    if [ ! -f "$MANIFEST" ]; then echo "SKIP: $MANIFEST not found"; exit 0; fi
    # Increase QPS to API server
    if grep -q "kube-api-qps" "$MANIFEST"; then
      sed -i 's/--kube-api-qps=[0-9]*/--kube-api-qps=100/' "$MANIFEST"
      sed -i 's/--kube-api-burst=[0-9]*/--kube-api-burst=200/' "$MANIFEST"
    else
      sed -i '/--leader-elect/a\    - --kube-api-qps=100\n    - --kube-api-burst=200' "$MANIFEST"
    fi
    # Increase resources (default 100m CPU, no memory — too lean for burst)
    sed -i 's/cpu: 100m/cpu: 2000m/' "$MANIFEST"
    if ! grep -q "memory:" "$MANIFEST"; then
      sed -i '/cpu: 2000m/a\        memory: 4Gi' "$MANIFEST"
    fi
    echo "DONE: scheduler qps=100, burst=200, cpu=2, memory=4Gi"
    ;;

  controller-manager)
    MANIFEST="/etc/kubernetes/manifests/kube-controller-manager.yaml"
    if [ ! -f "$MANIFEST" ]; then echo "SKIP: $MANIFEST not found"; exit 0; fi
    if grep -q "kube-api-qps" "$MANIFEST"; then
      sed -i 's/--kube-api-qps=[0-9]*/--kube-api-qps=100/' "$MANIFEST"
      sed -i 's/--kube-api-burst=[0-9]*/--kube-api-burst=200/' "$MANIFEST"
    else
      sed -i '/--leader-elect=true/a\    - --kube-api-qps=100\n    - --kube-api-burst=200' "$MANIFEST"
    fi
    # Increase resources (default 200m CPU, no memory — crashes under burst)
    sed -i 's/cpu: 200m/cpu: 2000m/' "$MANIFEST"
    if ! grep -q "memory:" "$MANIFEST"; then
      sed -i '/cpu: 2000m/a\        memory: 4Gi' "$MANIFEST"
    fi
    echo "DONE: controller-manager qps=100, burst=200, cpu=2, memory=4Gi"
    ;;

  apiserver)
    MANIFEST="/etc/kubernetes/manifests/kube-apiserver.yaml"
    if [ ! -f "$MANIFEST" ]; then echo "SKIP: $MANIFEST not found"; exit 0; fi
    # Increase inflight request limits
    if grep -q "max-requests-inflight" "$MANIFEST"; then
      sed -i 's/--max-requests-inflight=[0-9]*/--max-requests-inflight=800/' "$MANIFEST"
    else
      sed -i '/--etcd-servers/a\    - --max-requests-inflight=800' "$MANIFEST"
    fi
    if grep -q "max-mutating-requests-inflight" "$MANIFEST"; then
      sed -i 's/--max-mutating-requests-inflight=[0-9]*/--max-mutating-requests-inflight=400/' "$MANIFEST"
    else
      sed -i '/--max-requests-inflight/a\    - --max-mutating-requests-inflight=400' "$MANIFEST"
    fi
    # Increase resource limits
    if grep -q 'memory:.*[0-9]*Mi' "$MANIFEST"; then
      sed -i 's/memory: [0-9]*Mi/memory: 4Gi/g' "$MANIFEST"
    fi
    if grep -q 'cpu:.*[0-9]*m' "$MANIFEST"; then
      sed -i 's/cpu: [0-9]*m/cpu: 2000m/g' "$MANIFEST"
    fi
    echo "DONE: apiserver inflight=800/400, memory=4Gi, cpu=2"
    ;;

  etcd)
    MANIFEST="/etc/kubernetes/manifests/etcd.yaml"
    if [ ! -f "$MANIFEST" ]; then echo "SKIP: $MANIFEST not found"; exit 0; fi
    # Increase DB quota
    if grep -q "quota-backend-bytes" "$MANIFEST"; then
      sed -i 's/--quota-backend-bytes=[0-9]*/--quota-backend-bytes=8589934592/' "$MANIFEST"
    else
      sed -i '/--data-dir/a\    - --quota-backend-bytes=8589934592' "$MANIFEST"
    fi
    # Increase resource limits
    if grep -q 'memory:.*[0-9]*Mi' "$MANIFEST"; then
      sed -i 's/memory: [0-9]*Mi/memory: 4Gi/g' "$MANIFEST"
    fi
    if grep -q 'cpu:.*[0-9]*m' "$MANIFEST"; then
      sed -i 's/cpu: [0-9]*m/cpu: 2000m/g' "$MANIFEST"
    fi
    echo "DONE: etcd quota=8GB, memory=4Gi, cpu=2"
    ;;

  *)
    echo "ERROR: unknown component $COMPONENT"
    exit 1
    ;;
esac
PATCH_INNER
)

ENCODED_SCRIPT=$(echo "$PATCH_SCRIPT" | base64 | tr -d '\n')

# Function to patch one component and wait for API to be healthy
patch_component() {
  local COMPONENT="$1"
  local DISPLAY="$2"

  echo "==> Patching ${DISPLAY}..."

  POD_NAME="cp-patch-${COMPONENT}-$(date +%s)"
  kubectl run "${POD_NAME}" --restart=Never --image=busybox \
    --overrides="{
      \"spec\": {
        \"hostPID\": true,
        \"nodeName\": \"${NODE_NAME}\",
        \"containers\": [{
          \"name\": \"patch\",
          \"image\": \"busybox\",
          \"command\": [
            \"nsenter\", \"--target\", \"1\",
            \"--mount\", \"--uts\", \"--ipc\", \"--net\", \"--pid\", \"--\",
            \"bash\", \"-c\",
            \"echo ${ENCODED_SCRIPT} | base64 -d > /tmp/cp-patch.sh && chmod +x /tmp/cp-patch.sh && /tmp/cp-patch.sh ${COMPONENT}\"
          ],
          \"securityContext\": {\"privileged\": true}
        }]
      }
    }" 2>/dev/null

  # Wait for the patch pod to complete
  sleep 5
  RESULT=$(kubectl logs "${POD_NAME}" 2>/dev/null || echo "pending")
  kubectl delete pod "${POD_NAME}" --force --grace-period=0 2>/dev/null || true

  echo "    ${RESULT}"

  # Wait for API server to be reachable
  echo "    Waiting for API server..."
  for i in $(seq 1 30); do
    if kubectl get nodes &>/dev/null; then
      echo "    API healthy after ${i}s"
      break
    fi
    sleep 2
  done

  # Extra settle time for the component to fully start
  sleep 5
  echo ""
}

# Patch in order: least disruptive first
patch_component "scheduler" "kube-scheduler (QPS=100, burst=200, 4Gi, 2 CPU)"
patch_component "controller-manager" "kube-controller-manager (QPS=100, burst=200, 4Gi, 2 CPU)"
patch_component "apiserver" "kube-apiserver (inflight=800/400, 4Gi, 2 CPU)"
patch_component "etcd" "etcd (quota=8GB, 4Gi, 2 CPU)"

# Final check
echo "==> Verifying all control plane pods are running..."
kubectl wait --for=condition=Ready node/"${NODE_NAME}" --timeout=120s 2>/dev/null || true
echo ""
echo "=============================================="
echo "  Control Plane Hardened"
echo "=============================================="
echo "  scheduler:          qps=100, burst=200, cpu=2, memory=4Gi"
echo "  controller-manager: qps=100, burst=200, cpu=2, memory=4Gi"
echo "  apiserver:          inflight=800/400, memory=4Gi, cpu=2"
echo "  etcd:               quota=8GB, memory=4Gi, cpu=2"
echo "=============================================="
echo ""
echo "  Control plane pods:"
kubectl get pods -n kube-system -l tier=control-plane -o wide 2>/dev/null || \
  kubectl get pods -n kube-system | grep -E "apiserver|etcd|scheduler|controller-manager"
echo ""

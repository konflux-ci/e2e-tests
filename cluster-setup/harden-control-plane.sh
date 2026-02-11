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
    # Leader election: tolerate longer API blips (default 10s renew is too tight under burst)
    if ! grep -q "leader-elect-lease-duration" "$MANIFEST"; then
      sed -i '/--leader-elect=true/a\    - --leader-elect-lease-duration=60s\n    - --leader-elect-renew-deadline=45s\n    - --leader-elect-retry-period=5s' "$MANIFEST"
    fi
    # Increase resources (default 100m CPU, no memory — too lean for burst)
    sed -i 's/cpu: 100m/cpu: 2000m/' "$MANIFEST"
    if ! grep -q "memory:" "$MANIFEST"; then
      sed -i '/cpu: 2000m/a\        memory: 4Gi' "$MANIFEST"
    fi
    echo "DONE: scheduler qps=100, burst=200, lease=60s/renew=45s, cpu=2, memory=4Gi"
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
    # Leader election: tolerate longer API blips (default 10s renew is too tight under burst)
    if ! grep -q "leader-elect-lease-duration" "$MANIFEST"; then
      sed -i '/--leader-elect=true/a\    - --leader-elect-lease-duration=60s\n    - --leader-elect-renew-deadline=45s\n    - --leader-elect-retry-period=5s' "$MANIFEST"
    fi
    # Increase resources (default 200m CPU, no memory — crashes under burst)
    sed -i 's/cpu: 200m/cpu: 2000m/' "$MANIFEST"
    if ! grep -q "memory:" "$MANIFEST"; then
      sed -i '/cpu: 2000m/a\        memory: 4Gi' "$MANIFEST"
    fi
    echo "DONE: controller-manager qps=100, burst=200, lease=60s/renew=45s, cpu=2, memory=4Gi"
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
    # Reduce event TTL: default 1h creates massive etcd write churn.
    # 30m is plenty for E2E debugging and halves event storage pressure.
    if ! grep -q "event-ttl" "$MANIFEST"; then
      sed -i '/--max-mutating-requests-inflight/a\    - --event-ttl=30m0s' "$MANIFEST"
    fi
    # Speed up bulk deletions (namespace cleanup) — default is 1 worker
    if ! grep -q "delete-collection-workers" "$MANIFEST"; then
      sed -i '/--event-ttl/a\    - --delete-collection-workers=5' "$MANIFEST"
    fi
    # Increase resource limits
    if grep -q 'memory:.*[0-9]*Mi' "$MANIFEST"; then
      sed -i 's/memory: [0-9]*Mi/memory: 4Gi/g' "$MANIFEST"
    fi
    if grep -q 'cpu:.*[0-9]*m' "$MANIFEST"; then
      sed -i 's/cpu: [0-9]*m/cpu: 2000m/g' "$MANIFEST"
    fi
    echo "DONE: apiserver inflight=800/400, event-ttl=30m, del-workers=5, memory=4Gi, cpu=2"
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
    # Auto-compaction: without this, etcd keeps ALL historical revisions.
    # Database grows unbounded, reads/writes slow down, 503s appear.
    # Periodic 5m compaction keeps the DB lean during burst workloads.
    if ! grep -q "auto-compaction" "$MANIFEST"; then
      sed -i '/--quota-backend-bytes/a\    - --auto-compaction-mode=periodic\n    - --auto-compaction-retention=5m' "$MANIFEST"
    fi
    # Increase resource limits — etcd needs more headroom than other components
    if grep -q 'memory:.*[0-9]*Mi' "$MANIFEST"; then
      sed -i 's/memory: [0-9]*Mi/memory: 8Gi/g' "$MANIFEST"
    fi
    if grep -q 'cpu:.*[0-9]*m' "$MANIFEST"; then
      sed -i 's/cpu: [0-9]*m/cpu: 3000m/g' "$MANIFEST"
    fi
    echo "DONE: etcd quota=8GB, auto-compact=5m, memory=8Gi, cpu=3"
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

  # Wait for the patch pod to complete (not a fixed sleep)
  echo "    Waiting for pod to complete..."
  for i in $(seq 1 30); do
    PHASE=$(kubectl get pod "${POD_NAME}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Pending")
    if [ "$PHASE" = "Succeeded" ] || [ "$PHASE" = "Failed" ]; then
      echo "    Pod finished (${PHASE}) after ${i}s."
      break
    fi
    sleep 1
  done
  RESULT=$(kubectl logs "${POD_NAME}" 2>/dev/null || echo "pending")
  kubectl delete pod "${POD_NAME}" --force --grace-period=0 2>/dev/null || true

  echo "    ${RESULT}"

  # Wait for API server to be fully reachable (up to 120s)
  echo "    Waiting for API server..."
  for i in $(seq 1 60); do
    if kubectl get nodes &>/dev/null; then
      echo "    API reachable after ${i}s"
      break
    fi
    sleep 2
  done

  # Wait for the specific component's pod to be Running and Ready
  # etcd label is "component=etcd", others are "component=kube-<name>"
  if [ "$COMPONENT" = "etcd" ]; then
    LABEL="component=etcd"
  else
    LABEL="component=kube-${COMPONENT}"
  fi
  echo "    Waiting for ${LABEL} pod to be 1/1 Running..."
  for i in $(seq 1 60); do
    POD_STATUS=$(kubectl get pods -n kube-system -l "${LABEL}" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
    POD_READY=$(kubectl get pods -n kube-system -l "${LABEL}" -o jsonpath='{.items[0].status.containerStatuses[0].ready}' 2>/dev/null || echo "false")
    if [ "$POD_STATUS" = "Running" ] && [ "$POD_READY" = "true" ]; then
      echo "    ${LABEL} Ready after ${i}s"
      break
    fi
    sleep 2
  done
  echo ""
}

# Patch in order: least disruptive first
patch_component "scheduler" "kube-scheduler (QPS=100, burst=200, lease=60s/45s, 4Gi, 2 CPU)"
patch_component "controller-manager" "kube-controller-manager (QPS=100, burst=200, lease=60s/45s, 4Gi, 2 CPU)"
patch_component "apiserver" "kube-apiserver (inflight=800/400, 4Gi, 2 CPU)"

# etcd last: wait extra for apiserver to be fully stable before touching its datastore
echo "    [pre-etcd] Ensuring apiserver is fully stable..."
for i in $(seq 1 10); do
  if kubectl get pods -n kube-system -l component=kube-apiserver -o jsonpath='{.items[0].status.containerStatuses[0].ready}' 2>/dev/null | grep -q true; then
    echo "    [pre-etcd] apiserver confirmed Ready"
    break
  fi
  sleep 2
done

patch_component "etcd" "etcd (quota=8GB, 8Gi, 3 CPU)"

# Final check
echo "==> Verifying all control plane pods are running..."
kubectl wait --for=condition=Ready node/"${NODE_NAME}" --timeout=120s 2>/dev/null || true
echo ""
echo "=============================================="
echo "  Control Plane Hardened"
echo "=============================================="
echo "  scheduler:          qps=100, burst=200, lease=60s/45s, cpu=2, memory=4Gi"
echo "  controller-manager: qps=100, burst=200, lease=60s/45s, cpu=2, memory=4Gi"
echo "  apiserver:          inflight=800/400, event-ttl=30m, del-workers=5, 4Gi, cpu=2"
echo "  etcd:               quota=8GB, auto-compact=5m, 8Gi, cpu=3"
echo "=============================================="
echo ""
echo "  Control plane pods:"
kubectl get pods -n kube-system -l tier=control-plane -o wide 2>/dev/null || \
  kubectl get pods -n kube-system | grep -E "apiserver|etcd|scheduler|controller-manager"
echo ""

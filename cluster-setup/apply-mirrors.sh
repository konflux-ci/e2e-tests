#!/bin/bash
# Apply registry mirror configuration to a KinD node.
# Usage: ./apply-mirrors.sh [KUBECONFIG] [NODE_NAME]
set -euo pipefail

KUBECONFIG="${1:-./kubeconfig}"
NODE_NAME="${2:-kind-mapt-control-plane}"
export KUBECONFIG

echo "==> Getting registry cache service IPs..."
QUAY_IP=$(kubectl get svc cache-quay -n registry-cache -o jsonpath='{.spec.clusterIP}')
GHCR_IP=$(kubectl get svc cache-ghcr -n registry-cache -o jsonpath='{.spec.clusterIP}')
CGR_IP=$(kubectl get svc cache-cgr -n registry-cache -o jsonpath='{.spec.clusterIP}')
REDHAT_IP=$(kubectl get svc cache-redhat -n registry-cache -o jsonpath='{.spec.clusterIP}')

echo "    quay=${QUAY_IP} ghcr=${GHCR_IP} cgr=${CGR_IP} redhat=${REDHAT_IP}"

# Generate the setup script
SETUP_SCRIPT=$(cat <<INNEREOF
#!/bin/bash
set -e
mkdir -p /etc/containerd/certs.d/quay.io /etc/containerd/certs.d/ghcr.io /etc/containerd/certs.d/cgr.dev /etc/containerd/certs.d/registry.access.redhat.com

cat > /etc/containerd/certs.d/quay.io/hosts.toml <<TOML
server = "https://quay.io"
[host."http://${QUAY_IP}:5000"]
  capabilities = ["pull", "resolve"]
[host."https://quay.io"]
  capabilities = ["pull", "resolve"]
TOML

cat > /etc/containerd/certs.d/ghcr.io/hosts.toml <<TOML
server = "https://ghcr.io"
[host."http://${GHCR_IP}:5000"]
  capabilities = ["pull", "resolve"]
[host."https://ghcr.io"]
  capabilities = ["pull", "resolve"]
TOML

cat > /etc/containerd/certs.d/cgr.dev/hosts.toml <<TOML
server = "https://cgr.dev"
[host."http://${CGR_IP}:5000"]
  capabilities = ["pull", "resolve"]
[host."https://cgr.dev"]
  capabilities = ["pull", "resolve"]
TOML

cat > /etc/containerd/certs.d/registry.access.redhat.com/hosts.toml <<TOML
server = "https://registry.access.redhat.com"
[host."http://${REDHAT_IP}:5000"]
  capabilities = ["pull", "resolve"]
[host."https://registry.access.redhat.com"]
  capabilities = ["pull", "resolve"]
TOML

if ! grep -q 'config_path' /etc/containerd/config.toml; then
  sed -i '/\[plugins."io.containerd.grpc.v1.cri".registry\]/a\\      config_path = "/etc/containerd/certs.d"' /etc/containerd/config.toml
fi

if grep -q 'max_concurrent_downloads' /etc/containerd/config.toml; then
  sed -i 's/max_concurrent_downloads.*/max_concurrent_downloads = 50/' /etc/containerd/config.toml
else
  sed -i '/\[plugins."io.containerd.grpc.v1.cri"\]/a\\    max_concurrent_downloads = 50' /etc/containerd/config.toml
fi

systemctl restart containerd

# Wait for containerd to fully stabilize after restart.
# With 200+ containers from previous runs, containerd needs time to recover
# its shim connections and bolt DB state. Restarting too early leads to crashes.
echo "Waiting for containerd to stabilize..."
sleep 5
for ATTEMPT in 1 2 3 4 5 6 7 8 9 10; do
  if crictl info 1>/dev/null 2>/dev/null; then
    echo "containerd healthy after attempt \${ATTEMPT}"
    break
  fi
  sleep 1
done
echo DONE
INNEREOF
)

# Base64 encode the script to avoid shell escaping issues
ENCODED_SCRIPT=$(echo "${SETUP_SCRIPT}" | base64 | tr -d '\n')

echo "==> Applying mirror config and restarting containerd on ${NODE_NAME}..."

kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: setup-mirrors
  namespace: default
spec:
  hostPID: true
  nodeName: ${NODE_NAME}
  restartPolicy: Never
  containers:
  - name: setup
    image: busybox
    securityContext:
      privileged: true
    command:
    - nsenter
    - "--target"
    - "1"
    - "--mount"
    - "--uts"
    - "--ipc"
    - "--net"
    - "--pid"
    - "--"
    - bash
    - "-c"
    - "echo ${ENCODED_SCRIPT} | base64 -d | bash"
EOF

echo "==> Waiting for setup to complete..."
sleep 20

RESULT=$(kubectl logs setup-mirrors 2>/dev/null || echo "still running...")
echo "    Result: ${RESULT}"

kubectl delete pod setup-mirrors --force --grace-period=0 2>/dev/null || true
kubectl wait --for=condition=Ready "node/${NODE_NAME}" --timeout=120s 2>/dev/null

echo "==> Mirror configuration applied!"

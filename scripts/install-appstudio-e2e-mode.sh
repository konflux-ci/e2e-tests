#!/bin/bash
#
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

#!/bin/bash

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")));

# Available openshift ci environments https://docs.ci.openshift.org/docs/architecture/step-registry/#available-environment-variables
export ARTIFACTS_DIR=${ARTIFACT_DIR:-"/tmp/appstudio"}

export OFFLINE_TOKEN=$(cat /usr/local/ci-secrets/redhat-appstudio-qe/offline_sso_token)
export KCP_KUBECONFIG_SECRET="/usr/local/ci-secrets/redhat-appstudio-qe/kcp_kubeconfig"
export GITHUB_TOKEN=$(cat /usr/local/ci-secrets/redhat-appstudio-qe/github-token)
export APPSTUDIO_WORKSPACE=ci-ap-752387ds

mkdir -p $HOME/.configs
cp "/usr/local/ci-secrets/redhat-appstudio-qe/kcp_kubeconfig" $HOME/.configs
chmod -R 755 $HOME/.configs
ls -larth $HOME/.configs
export KCP_KUBECONFIG="$HOME/.configs/kcp_kubeconfig"

(
set -x; cd "$(mktemp -d)" &&
OS="$(uname | tr '[:upper:]' '[:lower:]')" &&
ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" &&
KREW="krew-${OS}_${ARCH}" &&
curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/${KREW}.tar.gz" &&
tar zxvf "${KREW}.tar.gz" &&
./"${KREW}" install krew
)

export PATH="${KREW_ROOT:-$HOME/.krew}/bin:$PATH"
kubectl krew install oidc-login

git clone -b v0.8.2 https://github.com/kcp-dev/kcp
cd kcp
go version
go mod vendor
make install WHAT='./cmd/kubectl-kcp ./cmd/kubectl-workspaces ./cmd/kubectl-ws'
bash <(curl -s https://gist.githubusercontent.com/flacatus/30f9f2b64676421e676c9448d2fd9b0b/raw/9592ccfb18dd80b47bc9d255ed3ed57b2ab1e1ab/CLOUD_SERVICES%2520oauth%2520token%2520robot)
cd ..
export CLUSTER_KUBECONFIG=$KUBECONFIG
export KUBECONFIG=$KCP_KUBECONFIG

kubectl config get-contexts
kubectl kcp workspace use '~'
APPSTUDIO_ROOT=$(kubectl kcp workspace . --short)

export WORKSPACE_ID=$(date +%s)

cat > hack/preview.env << EOF
export MY_GIT_FORK_REMOTE="qe"
export CLUSTER_KUBECONFIG="$CLUSTER_KUBECONFIG"
export KCP_KUBECONFIG="$KCP_KUBECONFIG"
export MY_GITHUB_ORG="redhat-appstudio-qe"
export MY_GITHUB_TOKEN=$GITHUB_TOKEN
export COMPUTE="appstudio-0123"
export ROOT_WORKSPACE=$APPSTUDIO_ROOT
export APPSTUDIO_WORKSPACE=ci-$WORKSPACE_ID
EOF

git clone https://$GITHUB_TOKEN@github.com/redhat-appstudio/infra-deployments.git "$WORKSPACE"/tmp/infra-deployments
cd "$WORKSPACE"/tmp/infra-deployments


cat hack/preview.env
echo "End"
/bin/bash hack/bootstrap.sh -m preview

/bin/bash hack/destroy.sh --kcp-kubeconfig $KCP_KUBECONFIG --cluster-kubeconfig $CLUSTER_KUBECONFIG

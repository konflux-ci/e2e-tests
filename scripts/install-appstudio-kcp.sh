#!/bin/bash
#
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

command -v kubectl >/dev/null 2>&1 || { echo "kubectl is not installed. Aborting."; exit 1; }
command -v oc >/dev/null 2>&1 || { echo "oc cli is not installed. Aborting."; exit 1; }

export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}
export MY_GIT_FORK_REMOTE="qe"
export MY_GITHUB_ORG="redhat-appstudio-qe"
export WORKSPACE_ID=$(tr -dc a-z0-9 </dev/urandom | head -c 5 ; echo '')
export TEST_BRANCH_ID="$(date +%s)"

# KCP related environments
export KCP_CONTEXT=
export KCP_KUBECONFIG=
export CLUSTER_KUBECONFIG=
export ROOT_WORKSPACE=
export IS_STABLE="false"

while [[ $# -gt 0 ]]
do
    case "$1" in
        -kc|--kcp-context)
            export KCP_CONTEXT=$2
            ;;
        -kk|--kcp-kubeconfig)
            export KCP_KUBECONFIG=$2
            echo $2
            
            ;;
        -ck|--cluster-kubeconfig)
            export CLUSTER_KUBECONFIG=$2
            ;;
        -s|--stable)
            export IS_STABLE="true"
            echo -e "$IS_STABLE"
            exit 0
            ;;
        *)
            ;;
    esac
    shift
done

if [[ -z "$KCP_CONTEXT" ]];then
    echo "[ERROR] Not KCP context defined in the script. Please use flag '-kc' or '--kcp-context' to define the kcp context." 
    exit 1
fi

if [[ -z "$KCP_KUBECONFIG" ]];then
    echo "[ERROR] KCP kubeconfig not defined. Please use flag '-kk' or '--kcp-kubeconfig' to define the kcp kubeconfig." 
    exit 1
fi

if [[ -z "$CLUSTER_KUBECONFIG" ]];then
    echo "[ERROR] Not cluster kubeconfig defined. Please use flag '-ck' or '--cluster-kubeconfig' to define the kcp physical cluster target kubeconfig." 
    exit 1
fi

if [[ -z "$OFFLINE_TOKEN" ]];then
    echo "[ERROR] Not cluster kubeconfig defined. Please use flag '-ck' or '--cluster-kubeconfig' to define the kcp physical cluster target kubeconfig." 
    exit 1
fi

# Installing oidc-login plugin for kubectl. More information about oidc-login plugin can be found here: https://github.com/int128/kubelogin.
# oidc-login will be used to authenticate using SSO against KCP
function installKubectlOIDCLoginPlugin() {
    echo -e "[INFO] Installing krew for oidc-login plugin installation."
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
}

# Some explanation
function installKCPKubectlPlugins() {
    echo -e "Install"
}

# Some explanation
function redHatSSOAuthentication() {
    # First we need to use the offline token to obtain an access token from
    # sso.redhat.com:
    local sso_token_request=$(
        curl \
        --silent \
        --header "Accept: application/json" \
        --header "Content-Type: application/x-www-form-urlencoded" \
        --data-urlencode "grant_type=refresh_token" \
        --data-urlencode "client_id=cloud-services" \
        --data-urlencode "refresh_token=${OFFLINE_TOKEN}" \
        "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token" \
        | \
        jq --raw-output "."
    )

    local access_token=$(echo $sso_token_request | jq .access_token)
    local refresh_token=$(echo $sso_token_request | jq .refresh_token)

    cat <<EOF > de0b44c30948a686e739661da92d5a6bf9c6b1fb85ce4c37589e089ba03d0ec6
    {"id_token":${access_token},"refresh_token":${refresh_token}}
EOF

    mkdir -p ~/.kube/cache/oidc-login/ && cp -f de0b44c30948a686e739661da92d5a6bf9c6b1fb85ce4c37589e089ba03d0ec6 ~/.kube/cache/oidc-login/
}

function checkIfKCPContextExists() {
    kubectl config use-context $KCP_CONTEXT
}

function createAppStudioRootWorkspace() {
    kubectl kcp workspace use '~'
    kubectl kcp workspace create ci-$WORKSPACE_ID --enter
    export APPSTUDIO_ROOT="$(kubectl ws . --short)"
}

# Download gitops repository to install AppStudio in e2e mode.
function cloneInfraDeployments() {
    if [ -d $WORKSPACE"/tmp/infra-deployments" ] 
    then
        echo -e "[INFO] tmp/infra-deployments already exists. Deleting..." 
        rm -rf $WORKSPACE"/tmp/infra-deployments"
    fi

    git clone https://github.com/redhat-appstudio/infra-deployments "$WORKSPACE"/tmp/infra-deployments
    cd "$WORKSPACE"/tmp/infra-deployments
}

function preview() {
    cat > "$WORKSPACE"/tmp/infra-deployments/hack/preview.env << EOF
export CLUSTER_KUBECONFIG="$CLUSTER_KUBECONFIG"
export KCP_KUBECONFIG="$KCP_KUBECONFIG"
export ROOT_WORKSPACE="$APPSTUDIO_ROOT"
EOF
}

function install() {
    git remote add "${MY_GIT_FORK_REMOTE}" https://github.com/"${MY_GITHUB_ORG}"/infra-deployments.git
    "$WORKSPACE"/tmp/infra-deployments/hack/bootstrap.sh -m preview
}

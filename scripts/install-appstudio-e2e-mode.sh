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

export MY_GIT_FORK_REMOTE="qe"
export MY_GITHUB_ORG=${GITHUB_E2E_ORGANIZATION:-"redhat-appstudio-qe"}
export MY_GITHUB_TOKEN="${GITHUB_TOKEN}"
export TEST_BRANCH_ID=$(date +%s)
export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}
export E2E_APPLICATIONS_NAMESPACE=${E2E_APPLICATIONS_NAMESPACE:-appstudio-e2e-test}

# Download gitops repository to install AppStudio in e2e mode.
function cloneInfraDeployments() {
    git clone https://github.com/redhat-appstudio/infra-deployments.git "$WORKSPACE"/tmp/infra-deployments
}

# Add a custom remote for infra-deployments repository.
function addQERemoteForkAndInstallAppstudio() {
    cd "$WORKSPACE"/tmp/infra-deployments
    git remote add "${MY_GIT_FORK_REMOTE}" https://github.com/"${MY_GITHUB_ORG}"/infra-deployments.git

    # Start AppStudio installation
    /bin/bash hack/bootstrap-cluster.sh preview
    cd "$WORKSPACE"
}

# Add a custom remote for infra-deployments repository.
function initializeSPIVault() {
   curl https://raw.githubusercontent.com/redhat-appstudio/e2e-tests/main/scripts/spi-e2e-setup.sh | bash -s
}

# Secrets used by pipelines to push component containers to quay.io
function createApplicationServiceSecrets() {
    echo -e "[INFO] Creating application-service related secrets in $E2E_APPLICATIONS_NAMESPACE namespace"

    echo "$QUAY_TOKEN" | base64 --decode > docker.config
    oc create namespace "$E2E_APPLICATIONS_NAMESPACE" --dry-run=client -o yaml | oc apply -f - || true
    kubectl create secret docker-registry redhat-appstudio-registry-pull-secret -n  "$E2E_APPLICATIONS_NAMESPACE" --from-file=.dockerconfigjson=docker.config || true
    kubectl create secret docker-registry redhat-appstudio-staginguser-pull-secret -n "$E2E_APPLICATIONS_NAMESPACE" --from-file=.dockerconfigjson=docker.config || true
    rm docker.config
}

# Install Toolchain (Sandbox) Operators
function installToolchainSandboxOperators() {
    cd "$WORKSPACE"/tmp/infra-deployments
    /bin/bash hack/sandbox-development-mode.sh
    cd "$WORKSPACE"
}

# Install MultiCluster Engine
function installMultiClusterEngine() {
    cd "$WORKSPACE"
    /bin/bash scripts/singapore-setup.sh
    cd "$WORKSPACE"
}

while [[ $# -gt 0 ]]
do
    case "$1" in
        install)
            cloneInfraDeployments
            addQERemoteForkAndInstallAppstudio
            createApplicationServiceSecrets
            installToolchainSandboxOperators
            installMultiClusterEngine
            initializeSPIVault
            ;;
        *)
            ;;
    esac
    shift
done

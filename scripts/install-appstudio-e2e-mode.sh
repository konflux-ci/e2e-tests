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
export MY_GITHUB_ORG=${MY_GITHUB_ORG:-"redhat-appstudio-qe"}
export MY_GITHUB_TOKEN="${GITHUB_TOKEN}"
export TEST_BRANCH_ID=$(date +%s)
export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")));

# Download gitops repository to install AppStudio in e2e mode.
function cloneInfraDeployments() {
    git clone https://github.com/redhat-appstudio/infra-deployments.git ./tmp/infra-deployments
}

# Add a custom remote for infra-deployments repository.
function addQERemoteForkAndInstallAppstudio() {
    cd "$WORKSPACE"/tmp/infra-deployments
    git remote add "${MY_GIT_FORK_REMOTE}" https://github.com/"${MY_GITHUB_ORG}"/infra-deployments.git

    # Start AppStudio installation
    /bin/bash hack/bootstrap-cluster.sh preview
    cd "$WORKSPACE"
}

# See https://github.com/redhat-appstudio/infra-deployments#bootstrap-app-studio to get more info about preview
function installAppStudioE2EMode() {
    /bin/bash "$WORKSPACE"/tmp/infra-deployments/hack/bootstrap-cluster.sh preview
}

# Secrets used by pipelines to push component containers to quay.io
function createApplicationServiceSecrets() {
    echo -e "[INFO] Creating application-service related secrets"

    echo "$QUAY_TOKEN" | base64 --decode > docker.config
    oc create namespace application-service --dry-run=client -o yaml | oc apply -f - || true
    kubectl create secret docker-registry redhat-appstudio-registry-pull-secret -n  application-service --from-file=.dockerconfigjson=docker.config || true
    kubectl create secret docker-registry redhat-appstudio-staginguser-pull-secret -n  application-service --from-file=.dockerconfigjson=docker.config || true
    rm docker.config
}

while [[ $# -gt 0 ]]
do
    case "$1" in
        --install)
            cloneInfraDeployments
            addQERemoteForkAndInstallAppstudio
            createApplicationServiceSecrets
            ;;
        *)
            ;;
    esac
    shift
done

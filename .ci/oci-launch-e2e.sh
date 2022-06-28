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

function executeE2ETests() {
    make build
    "${WORKSPACE}"/bin/e2e-appstudio --ginkgo.junit-report="${ARTIFACTS_DIR}"/e2e-report.xml --ginkgo.progress --ginkgo.v
}

function prepareWebhookVariables() {
    #Export variables
    export webhook_salt=123456789
    export webhook_target=https://smee.io/JgVqn2oYFPY1CF
    export webhook_repositoryURL=https://github.com/$REPO_OWNER/$REPO_NAME
    export webhook_repositoryFullName=$REPO_OWNER/$REPO_NAME
    export webhook_pullNumber=$PULL_NUMBER
    # Rewrite variables in webhookConfig.yml
    curl https://raw.githubusercontent.com/redhat-appstudio/e2e-tests/main/webhookConfig.yml | envsubst > webhookConfig.yml
}

prepareWebhookVariables
executeE2ETests

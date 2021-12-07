#!/bin/bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")));

# catch and stop execution on any error
trap "catchFinishedCode" EXIT SIGINT

# Catch an error after existing from jenkins Workspace
function catchFinishedCode() {
    EXIT_CODE=$?

    if [ "$EXIT_CODE" == "1" ]; then
      echo "[ERROR] Failed to validate e2e tests against Red Hat App Studio. Please check Openshift CI logs"
    fi

    exit $EXIT_CODE
}

function installRedHatAppStudio() {
    git clone https://github.com/redhat-appstudio/infra-deployments.git
    "${WORKSPACE}"/infra-deployments/hack/bootstrap-cluster.sh
}

function runE2ETests() {
    make build
    make run
}

installRedHatAppStudio
runE2ETests

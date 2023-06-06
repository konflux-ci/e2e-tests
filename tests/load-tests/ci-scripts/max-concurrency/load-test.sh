#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# shellcheck disable=SC1090
source "/usr/local/ci-secrets/redhat-appstudio-load-test/load-test-scenario.${1:-concurrent}"

pushd "${2:-./tests/load-tests}"

export DOCKER_CONFIG_JSON QUAY_E2E_ORGANIZATION MY_GITHUB_ORG GITHUB_TOKEN
DOCKER_CONFIG_JSON=$QUAY_TOKEN
QUAY_E2E_ORGANIZATION=$(cat /usr/local/ci-secrets/redhat-appstudio-load-test/quay-org)
MY_GITHUB_ORG=$(cat /usr/local/ci-secrets/redhat-appstudio-load-test/github-org)

./run-max-concurrency.sh

popd

#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# shellcheck disable=SC1090
source "/usr/local/ci-secrets/redhat-appstudio-load-test/load-test-scenario.${1:-concurrent}"

pushd "${2:-./tests/load-tests}"

export QUAY_E2E_ORGANIZATION MY_GITHUB_ORG GITHUB_TOKEN TEKTON_PERF_ENABLE_PROFILING TEKTON_PERF_PROFILE_CPU_PERIOD
QUAY_E2E_ORGANIZATION=$(cat /usr/local/ci-secrets/redhat-appstudio-load-test/quay-org)
MY_GITHUB_ORG=$(cat /usr/local/ci-secrets/redhat-appstudio-load-test/github-org)

./run-max-concurrency.sh

popd

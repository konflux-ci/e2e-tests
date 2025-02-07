#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

date -Ins --utc >started
go run loadtest.go \
    --applications-count "${APPLICATIONS_COUNT:-1}" \
    --build-pipeline-selector-bundle "${BUILD_PIPELINE_SELECTOR_BUNDLE:-}" \
    --component-repo "${COMPONENT_REPO:-https://github.com/devfile-samples/devfile-sample-code-with-quarkus}" \
    --component-repo-revision "${COMPONENT_REPO_REVISION:-main}" \
    --components-count "${COMPONENTS_COUNT:-1}" \
    --concurrency "${CONCURRENCY:-1}" \
    --journey-duration "${JOURNEY_DURATION:-1h}" \
    --journey-repeats "${JOURNEY_REPEATS:-1}" \
    --log-info \
    --pipeline-repo-templating="${PIPELINE_REPO_TEMPLATING:-false}" \
    --output-dir "${OUTPUT_DIR:-.}" \
    --purge="${PURGE:-true}" \
    --quay-repo "${QUAY_REPO:-redhat-user-workloads-stage}" \
    --test-scenario-git-url "${TEST_SCENARIO_GIT_URL:-https://github.com/konflux-ci/integration-examples.git}" \
    --test-scenario-path-in-repo "${TEST_SCENARIO_PATH_IN_REPO:-pipelines/integration_resolver_pipeline_pass.yaml}" \
    --test-scenario-revision "${TEST_SCENARIO_REVISION:-main}" \
    --username "${USER_PREFIX:-undef}" \
    --waitintegrationtestspipelines="${WAIT_INTEGRATION_TESTS:-true}" \
    --waitpipelines="${WAIT_PIPELINES:-true}" \
    --stage
date -Ins --utc >ended

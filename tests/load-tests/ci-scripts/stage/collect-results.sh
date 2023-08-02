#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

ARTIFACT_DIR=${ARTIFACT_DIR:-.artifacts}

mkdir -p ${ARTIFACT_DIR}

pushd "${1:-./tests/load-tests}"

echo "Collecting load test results"
cp -vf *.log "${ARTIFACT_DIR}"
cp -vf load-tests.json "${ARTIFACT_DIR}"

popd

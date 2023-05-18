#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# shellcheck disable=SC1090
source "/usr/local/ci-secrets/redhat-appstudio-load-test/load-test-scenario.${1:-concurrent}"

pushd "${2:-.}"

# Install app-studio and tweak cluster configuration
echo "Installing app-studio and tweaking cluster configuration"
go mod tidy
go mod vendor
export MY_GITHUB_ORG QUAY_E2E_ORGANIZATION
MY_GITHUB_ORG=$(cat /usr/local/ci-secrets/redhat-appstudio-load-test/github-org)
QUAY_E2E_ORGANIZATION=$(cat /usr/local/ci-secrets/redhat-appstudio-load-test/quay-org)
make local/cluster/prepare

oc patch -n application-service secret has-github-token --type=json -p='[{"op": "add", "path": "/data/tokens", "value": "'"$(base64 -w0 </usr/local/ci-secrets/redhat-appstudio-load-test/github_accounts)"'"}]'
oc patch -n application-service secret has-github-token -p '{"data": {"token": null}}'
oc rollout restart deployment -n application-service
oc rollout status deployment -n application-service -w

popd

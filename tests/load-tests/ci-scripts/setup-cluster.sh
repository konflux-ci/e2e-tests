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
export MY_GITHUB_ORG QUAY_E2E_ORGANIZATION INFRA_DEPLOYMENTS_ORG INFRA_DEPLOYMENTS_BRANCH TEKTON_PERF_THREADS_PER_CONTROLLER TEKTON_PERF_KUBE_API_QPS TEKTON_PERF_KUBE_API_BURST
MY_GITHUB_ORG=$(cat /usr/local/ci-secrets/redhat-appstudio-load-test/github-org)
QUAY_E2E_ORGANIZATION=$(cat /usr/local/ci-secrets/redhat-appstudio-load-test/quay-org)
INFRA_DEPLOYMENTS_ORG=$MY_GITHUB_ORG
INFRA_DEPLOYMENTS_BRANCH=tekton-tuning-$(mktemp -u XXXX)
TEKTON_PERF_THREADS_PER_CONTROLLER=${TEKTON_PERF_THREADS_PER_CONTROLLER:-2}
TEKTON_PERF_KUBE_API_QPS=${TEKTON_PERF_KUBE_API_QPS:-5}
TEKTON_PERF_KUBE_API_BURST=${TEKTON_PERF_KUBE_API_BURST:-10}

## Tweak infra-deployments
echo "Tweaking infra-deployments"
infra_deployment_dir=$(mktemp -d)
git clone --branch main "https://${GITHUB_TOKEN}@github.com/redhat-appstudio/infra-deployments.git" "$infra_deployment_dir"
envsubst <tests/load-tests/ci-scripts/tekton-performance/update-tekton-config-performance.yaml >"$infra_deployment_dir/components/pipeline-service/development/update-tekton-config-performance.yaml"
pushd "$infra_deployment_dir"
git checkout -b "$INFRA_DEPLOYMENTS_BRANCH" origin/main
git add "$infra_deployment_dir/components/pipeline-service/development/update-tekton-config-performance.yaml"
git commit -m "WIP: tekton performance tuning"
git remote add tekton-tuning "https://${GITHUB_TOKEN}@github.com/$INFRA_DEPLOYMENTS_ORG/infra-deployments.git"
git push -u tekton-tuning "$INFRA_DEPLOYMENTS_BRANCH"
popd
rm -rf "$infra_deployment_dir"

## Install infra-deployments
echo "Installing infra-deployments"
make local/cluster/prepare

## Patch HAS github secret
echo "Patching HAS github tokens"
oc patch -n application-service secret has-github-token --type=json -p='[{"op": "add", "path": "/data/tokens", "value": "'"$(base64 -w0 </usr/local/ci-secrets/redhat-appstudio-load-test/github_accounts)"'"}]'
oc patch -n application-service secret has-github-token -p '{"data": {"token": null}}'
oc rollout restart deployment -n application-service
oc rollout status deployment -n application-service -w

popd

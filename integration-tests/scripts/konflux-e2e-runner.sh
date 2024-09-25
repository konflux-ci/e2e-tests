#!/bin/bash

# Function to apply Docker rate limit workaround for a given namespace and service account in ROSA HCP clusters.
docker_io_workaround() {
    local namespace=$1
    local service_account=$2

    oc create namespace $namespace --dry-run=client -o yaml | oc apply -f -
    oc create sa $service_account -n $namespace --dry-run=client -o yaml | oc apply -f -

    oc get secret/pull-secret -n openshift-config --template='{{index .data ".dockerconfigjson" | base64decode}}' > ./global-pull-secret.json
    oc get secret -n openshift-config -o yaml pull-secret > ./global-pull-secret-$namespace.yaml
    yq -i ".metadata.namespace = \"$namespace\"" ./global-pull-secret-$namespace.yaml

    oc registry login --registry=docker.io --auth-basic="$DOCKER_IO_AUTH" --to=./global-pull-secret.json
    oc apply -f ./global-pull-secret-$namespace.yaml -n $namespace
    oc set data secret/pull-secret -n $namespace --from-file=.dockerconfigjson=./global-pull-secret.json
    oc secrets link $service_account pull-secret --for=pull -n $namespace
}

# List of namespaces and corresponding service accounts
declare -A namespace_service_account=(
    ["konflux-otel"]="open-telemetry-opentelemetry-collector"
    ["minio-operator"]="minio-operator"
)

# Apply workaround for each namespace and service account
for namespace in "${!namespace_service_account[@]}"; do
    docker_io_workaround $namespace ${namespace_service_account[$namespace]}
done

# Prepare git, pair branch if necessary, Install Konflux and run e2e tests
cd "$(mktemp -d)"

git config --global user.name "redhat-appstudio-qe-bot"
git config --global user.email redhat-appstudio-qe-bot@redhat.com

mkdir -p "${HOME}/creds"
git_creds_path="${HOME}/creds/file"
git config --global credential.helper "store --file $git_creds_path"
echo "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com" > "$git_creds_path"

git clone --origin upstream --branch main "https://${GITHUB_TOKEN}@github.com/konflux-ci/e2e-tests.git" .
make ci/prepare/e2e-branch 2>&1 | tee "${ARTIFACT_DIR}"/e2e-branch.log
make ci/test/e2e 2>&1 | tee "${ARTIFACT_DIR}"/e2e-tests.log

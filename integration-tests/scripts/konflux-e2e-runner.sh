#!/bin/bash

set -e
set -o pipefail

function postActions() {
    local EXIT_CODE=$?

    # Unregister PaC Server from SprayProxy
    if [ "$UNREGISTER_PAC" == "true" ]; then
        make ci/sprayproxy/unregister || true
    fi

    # Save artifacts
    cd /workspace || exit 1
    oras login -u "$ORAS_USERNAME" -p "$ORAS_PASSWORD" quay.io
    echo '{"doc": "README.md"}' > config.json

    oras push "$ORAS_CONTAINER" --config config.json:application/vnd.acme.rocket.config.v1+json \
        ./test-artifacts/:application/vnd.acme.rocket.docs.layer.v1+tar

    [[ "${EXIT_CODE}" != "0" ]] && echo "[ERROR] Job failed." || echo "[INFO] Job completed successfully."

    exit "$EXIT_CODE"
}

trap postActions EXIT

# Save the current kubeconfig and unset the default
oc config view --minify --raw > /workspace/kubeconfig
unset KUBECONFIG
export KUBECONFIG=/workspace/kubeconfig

# Create artifact directory if it doesn't exist
if [ ! -d "$ARTIFACT_DIR" ]; then
    mkdir -p "$ARTIFACT_DIR"
fi

# Export environment variables from secrets
export DEFAULT_QUAY_ORG=redhat-appstudio-qe
export DEFAULT_QUAY_ORG_TOKEN=$(cat /usr/local/konflux-ci-secrets/default-quay-org-token)
export GITHUB_USER=""
export GITHUB_TOKEN=""
export GITHUB_TOKENS_LIST=$(cat /usr/local/konflux-ci-secrets/github_accounts)
export QUAY_TOKEN=$(cat /usr/local/konflux-ci-secrets/quay-token)
export QUAY_OAUTH_USER=$(cat /usr/local/konflux-ci-secrets/quay-oauth-user)
export QUAY_OAUTH_TOKEN=$(cat /usr/local/konflux-ci-secrets/quay-oauth-token)
export PYXIS_STAGE_KEY=$(cat /usr/local/konflux-ci-secrets/pyxis-stage-key)
export PYXIS_STAGE_CERT=$(cat /usr/local/konflux-ci-secrets/pyxis-stage-cert)
export OFFLINE_TOKEN=$(cat /usr/local/konflux-ci-secrets/stage_offline_token)
export TOOLCHAIN_API_URL=$(cat /usr/local/konflux-ci-secrets/stage_toolchain_api_url)
export KEYLOAK_URL=$(cat /usr/local/konflux-ci-secrets/stage_keyloak_url)
export EXODUS_PROD_KEY=$(cat /usr/local/konflux-ci-secrets/exodus_prod_key)
export EXODUS_PROD_CERT=$(cat /usr/local/konflux-ci-secrets/exodus_prod_cert)
export CGW_USERNAME=$(cat /usr/local/konflux-ci-secrets/cgw_username)
export CGW_TOKEN=$(cat /usr/local/konflux-ci-secrets/cgw_token)
export REL_IMAGE_CONTROLLER_QUAY_ORG=$(cat /usr/local/konflux-ci-secrets/release_image_controller_quay_org)
export REL_IMAGE_CONTROLLER_QUAY_TOKEN=$(cat /usr/local/konflux-ci-secrets/release_image_controller_quay_token)
export OPENSHIFT_API=$(yq e '.clusters[0].cluster.server' "$KUBECONFIG")
export OPENSHIFT_USERNAME="kubeadmin"
export PREVIOUS_RATE_REMAINING=0
export QE_SPRAYPROXY_HOST=$(cat /usr/local/konflux-ci-secrets/qe-sprayproxy-host)
export QE_SPRAYPROXY_TOKEN=$(cat /usr/local/konflux-ci-secrets/qe-sprayproxy-token)
export E2E_PAC_GITHUB_APP_ID=$(cat /usr/local/konflux-ci-secrets/pac-github-app-id)
export E2E_PAC_GITHUB_APP_PRIVATE_KEY=$(cat /usr/local/konflux-ci-secrets/pac-github-app-private-key)
export PAC_GITHUB_APP_WEBHOOK_SECRET=$(cat /usr/local/konflux-ci-secrets/pac-github-app-webhook-secret)
export SLACK_BOT_TOKEN=$(cat /usr/local/konflux-ci-secrets/slack-bot-token)
export MULTI_PLATFORM_AWS_ACCESS_KEY=$(cat /usr/local/konflux-ci-secrets/multi-platform-aws-access-key)
export MULTI_PLATFORM_AWS_SECRET_ACCESS_KEY=$(cat /usr/local/konflux-ci-secrets/multi-platform-aws-secret-access-key)
export MULTI_PLATFORM_AWS_SSH_KEY=$(cat /usr/local/konflux-ci-secrets/multi-platform-aws-ssh-key)
export MULTI_PLATFORM_IBM_API_KEY=$(cat /usr/local/konflux-ci-secrets/multi-platform-ibm-api-key)
export ORAS_USERNAME=$(cat /usr/local/konflux-ci-secrets/oras-username)
export ORAS_PASSWORD=$(cat /usr/local/konflux-ci-secrets/oras-password)
export DOCKER_IO_AUTH=$(cat /usr/local/konflux-ci-secrets/docker_io)
export GITLAB_BOT_TOKEN=$(cat /usr/local/konflux-ci-secrets/gitlab-bot-token)

export ENABLE_SCHEDULING_ON_MASTER_NODES=false

# Process GitHub accounts
IFS=',' read -r -a GITHUB_ACCOUNTS_ARRAY <<< "$(cat /usr/local/konflux-ci-secrets/github_accounts)"
for account in "${GITHUB_ACCOUNTS_ARRAY[@]}"; do
    IFS=':' read -r -a GITHUB_USERNAME_ARRAY <<< "$account"
    
    GH_RATE_REMAINING=$(curl -s \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer ${GITHUB_USERNAME_ARRAY[1]}" \
        https://api.github.com/rate_limit | jq ".rate.remaining")

    echo -e "[INFO ] user: ${GITHUB_USERNAME_ARRAY[0]} with rate limit remaining $GH_RATE_REMAINING"
    if [[ "$GH_RATE_REMAINING" -ge "$PREVIOUS_RATE_REMAINING" ]]; then
        GITHUB_USER="${GITHUB_USERNAME_ARRAY[0]}"
        GITHUB_TOKEN="${GITHUB_USERNAME_ARRAY[1]}"
    fi
    PREVIOUS_RATE_REMAINING="$GH_RATE_REMAINING"
done

echo -e "[INFO] Start tests with user: ${GITHUB_USER}"

# ROSA HCP workaround for docker limits in konflux-otel
oc create namespace konflux-otel
oc create sa open-telemetry-opentelemetry-collector -n konflux-otel

oc get secret/pull-secret -n openshift-config --template='{{index .data ".dockerconfigjson" | base64decode}}' > ./global-pull-secret.json
oc get secret -n openshift-config -o yaml pull-secret > global-pull-secret.yaml

yq -i '.metadata.namespace = "konflux-otel"' global-pull-secret.yaml

oc registry login --registry=docker.io --auth-basic="$DOCKER_IO_AUTH" --to=./global-pull-secret.json
oc apply -f global-pull-secret.yaml -n konflux-otel
oc set data secret/pull-secret -n konflux-otel --from-file=.dockerconfigjson=./global-pull-secret.json
oc secrets link open-telemetry-opentelemetry-collector pull-secret --for=pull -n konflux-otel

# Prepare and run tests
cd "$(mktemp -d)" || exit 1

git config --global user.name "redhat-appstudio-qe-bot"
git config --global user.email redhat-appstudio-qe-bot@redhat.com

mkdir -p "${HOME}/creds"
GIT_CREDS_PATH="${HOME}/creds/file"
git config --global credential.helper "store --file ${GIT_CREDS_PATH}"

echo "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com" > "${GIT_CREDS_PATH}"

git clone --origin upstream --branch main "https://${GITHUB_TOKEN}@github.com/konflux-ci/e2e-tests.git" .

make ci/prepare/e2e-branch 2>&1 | tee /workspace/test-artifacts/e2e-branch.log

UNREGISTER_PAC=true
make ci/test/e2e 2>&1 | tee /workspace/test-artifacts/e2e-tests.log

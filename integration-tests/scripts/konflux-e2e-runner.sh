#!/bin/bash

set -euo pipefail

log() {
    echo -e "[$(date +'%Y-%m-%d %H:%M:%S')] [$1] $2"
}

load_envs() {

    declare -A config_envs=(
        [ENABLE_SCHEDULING_ON_MASTER_NODES]="false"
        [UNREGISTER_PAC]="true"
        [EC_DISABLE_DOWNLOAD_SERVICE]="true"
        [ARTIFACT_DIR]="$(mktemp -d)"
    )

    declare -A vault_secrets=(
        [DEFAULT_QUAY_ORG_TOKEN]="default-quay-org-token"
        [GITHUB_TOKENS_LIST]="github_accounts"
        [QUAY_TOKEN]="quay-token"
        [QUAY_OAUTH_USER]="quay-oauth-user"
        [QUAY_OAUTH_TOKEN]="quay-oauth-token"
        [PYXIS_STAGE_KEY]="pyxis-stage-key"
        [PYXIS_STAGE_CERT]="pyxis-stage-cert"
        [OFFLINE_TOKEN]="stage_offline_token"
        [TOOLCHAIN_API_URL]="stage_toolchain_api_url"
        [KEYLOAK_URL]="stage_keyloak_url"
        [EXODUS_PROD_KEY]="exodus_prod_key"
        [EXODUS_PROD_CERT]="exodus_prod_cert"
        [CGW_USERNAME]="cgw_username"
        [CGW_TOKEN]="cgw_token"
        [REL_IMAGE_CONTROLLER_QUAY_ORG]="release_image_controller_quay_org"
        [REL_IMAGE_CONTROLLER_QUAY_TOKEN]="release_image_controller_quay_token"
        [QE_SPRAYPROXY_HOST]="qe-sprayproxy-host"
        [QE_SPRAYPROXY_TOKEN]="qe-sprayproxy-token"
        [E2E_PAC_GITHUB_APP_ID]="pac-github-app-id"
        [E2E_PAC_GITHUB_APP_PRIVATE_KEY]="pac-github-app-private-key"
        [PAC_GITHUB_APP_WEBHOOK_SECRET]="pac-github-app-webhook-secret"
        [SLACK_BOT_TOKEN]="slack-bot-token"
        [MULTI_PLATFORM_AWS_ACCESS_KEY]="multi-platform-aws-access-key"
        [MULTI_PLATFORM_AWS_SECRET_ACCESS_KEY]="multi-platform-aws-secret-access-key"
        [MULTI_PLATFORM_AWS_SSH_KEY]="multi-platform-aws-ssh-key"
        [MULTI_PLATFORM_IBM_API_KEY]="multi-platform-ibm-api-key"
        [DOCKER_IO_AUTH]="docker_io"
        [GITLAB_BOT_TOKEN]="gitlab-bot-token"
    )

    for var in "${!config_envs[@]}"; do
        export "$var"="${config_envs[$var]}"
    done

    for var in "${!vault_secrets[@]}"; do
        local file="/usr/local/konflux-ci-secrets/${vault_secrets[$var]}"
        if [[ -f "$file" ]]; then
            export "$var"=$(cat "$file")
        else
            log "WARNING" "Secret file for $var not found at $file"
        fi
    done
}

post_actions() {
    local exit_code=$?
    [[ "${UNREGISTER_PAC}" == "true" ]] && make ci/sprayproxy/unregister || log "WARNING" "Failed to unregister PaC Server from SprayProxy."

    cd /workspace
    local temp_annotation_file
    temp_annotation_file=$(mktemp)

    if ! manifests=$(oras manifest fetch "${OCI_STORAGE_CONTAINER}" | jq .annotations); then
        log "ERROR" "Failed to fetch manifest from ${OCI_STORAGE_CONTAINER}"
        exit 1
    fi

    jq -n --argjson manifest "$manifests" '{ "$manifest": $manifest }' > "$temp_annotation_file"
    oras pull "${OCI_STORAGE_CONTAINER}"

    for attempt in {1..5}; do
        if oras push "$ORAS_CONTAINER" --username="${OCI_STORAGE_USERNAME}" --password="${OCI_STORAGE_TOKEN}" "${ARTIFACT_DIR}"/:application/vnd.acme.rocket.docs.layer.v1+tar; then
            break
        elif [[ $attempt -eq 5 ]]; then
            log "ERROR" "oras push failed after 5 attempts."
            exit 1
        else
            log "WARNING" "oras push failed. Retrying ($attempt/5)..."
            sleep 5
        fi
    done

    log "INFO" "Job ${exit_code} == 0 ? 'script completed successfully' : 'script failed'."
    exit "$exit_code"
}

trap post_actions EXIT

load_envs

oc config view --minify --raw > /workspace/kubeconfig
export KUBECONFIG=/workspace/kubeconfig

previous_rate_remaining=0
while IFS=':' read -r github_user github_token; do
    gh_rate_remaining=$(curl -s -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $github_token" https://api.github.com/rate_limit | jq ".rate.remaining")
    log "INFO" "user: $github_user with rate limit remaining $gh_rate_remaining"
    if [[ "$gh_rate_remaining" -ge "$previous_rate_remaining" ]]; then
        GITHUB_USER="$github_user"
        GITHUB_TOKEN="$github_token"
    fi
    previous_rate_remaining="$gh_rate_remaining"
done < /usr/local/konflux-ci-secrets/github_accounts

log "INFO" "Start tests with user: ${GITHUB_USER}"

# ROSA HCP workaround for Docker limits
oc create namespace konflux-otel
oc create sa open-telemetry-opentelemetry-collector -n konflux-otel

oc get secret/pull-secret -n openshift-config --template='{{index .data ".dockerconfigjson" | base64decode}}' > ./global-pull-secret.json
oc get secret -n openshift-config -o yaml pull-secret > global-pull-secret.yaml
yq -i '.metadata.namespace = "konflux-otel"' global-pull-secret.yaml

oc registry login --registry=docker.io --auth-basic="$DOCKER_IO_AUTH" --to=./global-pull-secret.json
oc apply -f global-pull-secret.yaml -n konflux-otel
oc set data secret/pull-secret -n konflux-otel --from-file=.dockerconfigjson=./global-pull-secret.json
oc secrets link open-telemetry-opentelemetry-collector pull-secret --for=pull -n konflux-otel

# Prepare git, pair branch if necessary, Install Konflux and run e2e tests
cd "$(mktemp -d)"

git config --global user.name "redhat-appstudio-qe-bot"
git config --global user.email redhat-appstudio-qe-bot@redhat.com

mkdir -p "${HOME}/creds"
git_creds_path="${HOME}/creds/file"
git config --global credential.helper "store --file $git_creds_path"
echo "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com" > "$git_creds_path"

git clone --origin upstream --branch main "https://${GITHUB_TOKEN}@github.com/konflux-ci/e2e-tests.git" .
make ci/prepare/e2e-branch 2>&1 | tee /workspace/test-artifacts/e2e-branch.log
make ci/test/e2e 2>&1 | tee /workspace/test-artifacts/e2e-tests.log

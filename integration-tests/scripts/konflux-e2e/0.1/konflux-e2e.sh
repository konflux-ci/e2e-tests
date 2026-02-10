#!/bin/bash

set -euo pipefail

load_envs() {
    local konflux_ci_secrets_file="/usr/local/konflux-ci-secrets"
    local konflux_infra_secrets_file="/usr/local/konflux-test-infra"

    declare -A config_envs=(
        [ENABLE_SCHEDULING_ON_MASTER_NODES]="false"
        [UNREGISTER_PAC]="true"
        [EC_DISABLE_DOWNLOAD_SERVICE]="true"
        [DEFAULT_QUAY_ORG]="redhat-appstudio-qe"
        [OCI_STORAGE_USERNAME]="$(jq -r '."quay-username"' ${konflux_infra_secrets_file}/oci-storage)"
        [OCI_STORAGE_TOKEN]="$(jq -r '."quay-token"' ${konflux_infra_secrets_file}/oci-storage)"
    )

    declare -A load_envs_from_file=(
        [DEFAULT_QUAY_ORG_TOKEN]="${konflux_ci_secrets_file}/default-quay-org-token"
        [QUAY_TOKEN]="${konflux_ci_secrets_file}/quay-token"
        [RELEASE_CATALOG_TA_QUAY_TOKEN]="${konflux_ci_secrets_file}/release-catalog-ta-quay-token"
        [QUAY_OAUTH_USER]="${konflux_ci_secrets_file}/quay-oauth-user"
        [QUAY_OAUTH_TOKEN]="${konflux_ci_secrets_file}/quay-oauth-token"
        [PYXIS_STAGE_KEY]="${konflux_ci_secrets_file}/pyxis-stage-key"
        [PYXIS_STAGE_CERT]="${konflux_ci_secrets_file}/pyxis-stage-cert"
        [ATLAS_STAGE_ACCOUNT]="${konflux_ci_secrets_file}/atlas-stage-account"
        [ATLAS_STAGE_TOKEN]="${konflux_ci_secrets_file}/atlas-stage-token"
        [ATLAS_AWS_ACCESS_KEY_ID]="${konflux_ci_secrets_file}/atlas-aws-access-key-id"
        [ATLAS_AWS_ACCESS_SECRET]="${konflux_ci_secrets_file}/atlas-aws-secret-access-key"
        [OFFLINE_TOKEN]="${konflux_ci_secrets_file}/stage_offline_token"
        [TOOLCHAIN_API_URL]="${konflux_ci_secrets_file}/stage_toolchain_api_url"
        [KEYLOAK_URL]="${konflux_ci_secrets_file}/stage_keyloak_url"
        [EXODUS_PROD_KEY]="${konflux_ci_secrets_file}/exodus_prod_key"
        [EXODUS_PROD_CERT]="${konflux_ci_secrets_file}/exodus_prod_cert"
        [CGW_USERNAME]="${konflux_ci_secrets_file}/cgw_username"
        [CGW_TOKEN]="${konflux_ci_secrets_file}/cgw_token"
        [REL_IMAGE_CONTROLLER_QUAY_ORG]="${konflux_ci_secrets_file}/release_image_controller_quay_org"
        [REL_IMAGE_CONTROLLER_QUAY_TOKEN]="${konflux_ci_secrets_file}/release_image_controller_quay_token"
        [QE_SPRAYPROXY_HOST]="${konflux_ci_secrets_file}/qe-sprayproxy-host"
        [QE_SPRAYPROXY_TOKEN]="${konflux_ci_secrets_file}/qe-sprayproxy-token"
        [E2E_PAC_GITHUB_APP_ID]="${konflux_ci_secrets_file}/pac-github-app-id"
        [E2E_PAC_GITHUB_APP_PRIVATE_KEY]="${konflux_ci_secrets_file}/pac-github-app-private-key"
        [PAC_GITHUB_APP_WEBHOOK_SECRET]="${konflux_ci_secrets_file}/pac-github-app-webhook-secret"
        [SLACK_BOT_TOKEN]="${konflux_ci_secrets_file}/slack-bot-token"
        [MULTI_PLATFORM_AWS_ACCESS_KEY]="${konflux_ci_secrets_file}/multi-platform-aws-access-key"
        [MULTI_PLATFORM_AWS_SECRET_ACCESS_KEY]="${konflux_ci_secrets_file}/multi-platform-aws-secret-access-key"
        [MULTI_PLATFORM_AWS_SSH_KEY]="${konflux_ci_secrets_file}/multi-platform-aws-ssh-key"
        [MULTI_PLATFORM_IBM_API_KEY]="${konflux_ci_secrets_file}/multi-platform-ibm-api-key"
        [DOCKER_IO_AUTH]="${konflux_ci_secrets_file}/docker_io"
        [GITLAB_BOT_TOKEN]="${konflux_ci_secrets_file}/gitlab-bot-token"
        [CODEBERG_BOT_TOKEN]="${konflux_ci_secrets_file}/codeberg-bot-token"
        [SMEE_CHANNEL]="${konflux_ci_secrets_file}/smee-channel"
    )

    for var in "${!config_envs[@]}"; do
        export "$var"="${config_envs[$var]}"
    done

    for var in "${!load_envs_from_file[@]}"; do
        local file="${load_envs_from_file[$var]}"
        if [[ -f "$file" ]]; then
            export "$var"="$(<"$file")"
        else
            log "ERROR" "Secret file for $var not found at $file"
        fi
    done
}

post_actions() {
    local exit_code=$?

    if [[ "$exit_code" == "124" ]]; then
        # Separate the error from the test log with new lines so it's more visible
        printf "\n\n" | tee -a "${ARTIFACT_DIR}/e2e-tests.log"
        log "ERROR" "The process for bootstrapping the cluster and running tests timed out after $E2E_TIMEOUT" | tee -a "${ARTIFACT_DIR}/e2e-tests.log"
    fi

    if [[ "${UNREGISTER_PAC}" == "true" ]]; then
        make ci/sprayproxy/unregister | tee "${ARTIFACT_DIR}"/sprayproxy-unregister.log
    fi

    exit "$exit_code"
}

trap post_actions EXIT

load_envs

oc config view --minify --raw > /workspace/kubeconfig
export KUBECONFIG=/workspace/kubeconfig

# ROSA HCP workaround for Docker limits
DOCKER_CONFIG_JSON_FILE=docker-config.json
oc registry login --registry=docker.io --auth-basic="$DOCKER_IO_AUTH" --to=./$DOCKER_CONFIG_JSON_FILE

namespace_sa_names=$(cat << 'EOF'
minio-operator|console-sa
minio-operator|minio-operator
product-kubearchive|default
tekton-logging|vector-tekton-logs-collector
tekton-results|storage-sa
tekton-results|postgres-postgresql
EOF
)
while IFS='|' read -r ns sa_name; do
    oc create namespace "$ns" --dry-run=client -o yaml | oc apply -f -
    oc create sa "$sa_name" -n "$ns" --dry-run=client -o yaml | oc apply -f -
    if ! oc get secret/pull-secret -n "$ns" &> /dev/null; then
        oc create secret docker-registry pull-secret --from-file=.dockerconfigjson=./$DOCKER_CONFIG_JSON_FILE -n "$ns"
    fi
    oc secrets link "$sa_name" pull-secret --for=pull -n "$ns"
done <<< "$namespace_sa_names"

timeout "$E2E_TIMEOUT" make ci/test/e2e 2>&1 | tee "${ARTIFACT_DIR}/e2e-tests.log"

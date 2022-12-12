#!/bin/bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u
VAULT_NAMESPACE=${VAULT_NAMESPACE:-spi-vault}
VAULT_PODNAME=${VAULT_PODNAME:-vault-0}

echo '[INFO] Deploying SPI OAuth2 config'

export OAUTH_URL='spi-oauth-route-spi-system.'$( oc get ingresses.config/cluster -o jsonpath={.spec.domain})
export tmpfile=$(mktemp -d)/config.yaml

# We are injecting the token manually for the e2e. No need a real secret for now
export SPI_GITHUB_CLIENT_ID="app-client-id"
export SPI_GITHUB_CLIENT_SECRET="app-secret"

# The legacy stuff can be removed once https://github.com/redhat-appstudio/infra-deployments/pull/638 is merged
# in infra-deployments
legacySpiConfig=$(cat <<EOF
sharedSecret: $(openssl rand -hex 20)
serviceProviders:
  - type: GitHub
    clientId: $SPI_GITHUB_CLIENT_ID
    clientSecret: $SPI_GITHUB_CLIENT_SECRET
  - type: Quay
    clientId: $SPI_GITHUB_CLIENT_ID
    clientSecret: $SPI_GITHUB_CLIENT_SECRET
baseUrl: https://spi-oauth-route-spi-system.$( oc get ingresses.config/cluster -o jsonpath={.spec.domain})
EOF
)

spiConfig=$(cat <<EOF
sharedSecret: $(openssl rand -hex 20)
serviceProviders:
  - type: GitHub
    clientId: $SPI_GITHUB_CLIENT_ID
    clientSecret: $SPI_GITHUB_CLIENT_SECRET
  - type: Quay
    clientId: $SPI_GITHUB_CLIENT_ID
    clientSecret: $SPI_GITHUB_CLIENT_SECRET
EOF
)

echo "Please go to https://github.com/settings/developers."
echo "And register new Github OAuth application for callback https://"$OAUTH_URL"/github/callback"

oc create namespace spi-system --dry-run=client -o yaml | oc apply -f -

# legacy config file
echo "$legacySpiConfig" > "$tmpfile"
oc create secret generic oauth-config \
    --save-config --dry-run=client \
    --from-file="$tmpfile" \
    -n spi-system \
    -o yaml |
oc apply -f -

# new-form config file
echo "$spiConfig" > "$tmpfile"
oc create secret generic shared-configuration-file \
    --save-config --dry-run=client \
    --from-file="$tmpfile" \
    -n spi-system \
    -o yaml |
oc apply -f -

rm "$tmpfile"

curl https://raw.githubusercontent.com/redhat-appstudio/service-provider-integration-operator/main/hack/vault-init.sh | VAULT_PODNAME=${VAULT_PODNAME} VAULT_NAMESPACE=${VAULT_NAMESPACE} bash -s

oc rollout restart deployment/spi-controller-manager -n spi-system
oc rollout restart deployment/spi-oauth-service -n spi-system

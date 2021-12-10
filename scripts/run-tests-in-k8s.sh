#!/bin/bash

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export WORKSPACE=$(dirname $(dirname $(readlink -f "$0")));

E2E_TEST_NAMESPACE=${1:-"appstudio-e2e"}
REPORT_DIR=${2:-"${WORKSPACE}/tmp"}

# Ensure there is no already existed project
oc delete namespace ${E2E_TEST_NAMESPACE} --wait=true --ignore-not-found || true

oc create namespace ${E2E_TEST_NAMESPACE}
oc project ${E2E_TEST_NAMESPACE}

ID=$(date +%s)
OPENSHIFT_API_URL=$(oc config view --minify -o jsonpath='{.clusters[*].cluster.server}')
OPENSHIFT_API_TOKEN=$(oc whoami -t)

TMP_POD_YML=$(mktemp)
TMP_KUBECONFIG_YML=$(mktemp)

cat "${WORKSPACE}/scripts/resources/kubeconfig.template.yaml" |
    sed -e "s#__OPENSHIFT_API_URL__#${OPENSHIFT_API_URL}#g" |
    sed -e "s#__OPENSHIFT_API_TOKEN__#${OPENSHIFT_API_TOKEN}#g" |
    cat > "${TMP_KUBECONFIG_YML}"

cat ${TMP_KUBECONFIG_YML}

oc delete configmap -n "${E2E_TEST_NAMESPACE}" e2e-appstudio-kubeconfig || true
oc create configmap -n "${E2E_TEST_NAMESPACE}" e2e-appstudio-kubeconfig \
    --from-file=config="${TMP_KUBECONFIG_YML}"

cat "${WORKSPACE}/scripts/resources/e2e-pod-template.yaml" |
    sed -e "s#__ID__#${ID}#g" |
    sed -e "s#__NAMESPACE__#${E2E_TEST_NAMESPACE}#g" |
    cat > "${TMP_POD_YML}"

cat "${TMP_POD_YML}"

# start the test
oc create -f "${TMP_POD_YML}"

# wait for the pod to start
while true; do
    sleep 3
    PHASE=$(oc get pod -n "${E2E_TEST_NAMESPACE}" e2e-appstudio-"${ID}" \
        --template='{{ .status.phase }}')
    if [[ "${PHASE}" == "Running" ]]; then
        break
    fi
done

# wait for the test to finish
oc logs -n "${E2E_TEST_NAMESPACE}" e2e-appstudio-"${ID}" -c e2e-test -f

# just to sleep
sleep 3

# download the test results
mkdir -p "${REPORT_DIR}/${ID}"

oc rsync -n "${E2E_TEST_NAMESPACE}" \
    e2e-appstudio-"${ID}":/test-run-results "${REPORT_DIR}/${ID}" -c download

oc exec -n "${E2E_TEST_NAMESPACE}" e2e-appstudio-"${ID}" -c download \
    -- touch /tmp/done

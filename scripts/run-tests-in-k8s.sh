#!/bin/bash

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export E2E_TEST_NAMESPACE="appstudio-e2e"
export REPORT_DIR="./tmp"
export GINKGO_SUITES=""
export ID=$(date +%s)

export OPENSHIFT_API_URL=$(oc config view --minify -o jsonpath='{.clusters[*].cluster.server}')
export OPENSHIFT_API_TOKEN=$(oc whoami -t)

while [[ $# -gt 0 ]]
do
    case "$1" in
        -n|--namespace)
            E2E_TEST_NAMESPACE=$2;
            ;;
        -r|--report)
            REPORT_DIR=$2;
            ;;
        -s|--suites)
            GINKGO_SUITES=$2
            ;;
        *)
            ;;
    esac
    shift  # Shift each argument out after processing them
done

echo "[INFO] Starting E2E AppStudio framework..."
echo "   Namespace   : ${E2E_TEST_NAMESPACE}"
echo "   Report-dir  : ${REPORT_DIR}"
echo "   Suites      : ${GINKGO_SUITES:-all}"
echo ""

# Ensure there is no already existed project and then create the project
oc delete namespace ${E2E_TEST_NAMESPACE} --wait=true --ignore-not-found || true
oc create namespace ${E2E_TEST_NAMESPACE}
oc project ${E2E_TEST_NAMESPACE}

# Create a temporary yaml with local kubeconfig. Is usefull to create the configmap with the kubeconfig to run the tests.
export TMP_KUBECONFIG_YAML=$(mktemp)
cat <<EOF > $TMP_KUBECONFIG_YAML
apiVersion: v1
clusters:
  - cluster:
      server: $OPENSHIFT_API_URL
      insecure-skip-tls-verify: true
    name: cluster
contexts:
  - context:
      cluster: cluster
      namespace: default
      user: kube:admin/cluster
    name: default/cluster/kube:admin
current-context: default/cluster/kube:admin
kind: Config
preferences: {}
users:
  - name: kube:admin/cluster
    user:
      token: $OPENSHIFT_API_TOKEN
EOF
cat "${TMP_KUBECONFIG_YAML}"

# Create configmap
oc delete configmap -n "${E2E_TEST_NAMESPACE}" e2e-appstudio-kubeconfig || true
oc create configmap -n "${E2E_TEST_NAMESPACE}" e2e-appstudio-kubeconfig \
    --from-file=config="${TMP_KUBECONFIG_YAML}"

# Run the test pod and also an additional container to download the results to local machine.
export POD_NAME=$(oc create -f - -o jsonpath='{.metadata.name}' <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: e2e-appstudio-$ID
  namespace: $E2E_TEST_NAMESPACE
spec:
  volumes:
    - name: test-run-results
    - name: kubeconfig
      configMap:
        name: e2e-appstudio-kubeconfig
  containers:
    - name: e2e-test
      image: quay.io/redhat-appstudio/e2e-tests:latest
      args:
        - "--ginkgo.junit-report=/test-run-results/report.xml"
        - "--ginkgo.focus=$GINKGO_SUITES"
      imagePullPolicy: Always
      env:
        - name: KUBECONFIG
          value: /tmp/kubeconfig/config
      volumeMounts:
        - name: test-run-results
          mountPath: /test-run-results
        - name: kubeconfig
          mountPath: /tmp/kubeconfig
    - name: download
      image: quay.io/crw_pr/rsync:latest
      volumeMounts:
        - name: test-run-results
          mountPath: /test-run-results
      command: ["sh"]
      args:
        [
          "-c",
          "while true; if [[ -f /tmp/done ]]; then exit 0; fi; do sleep 1; done",
        ]
  restartPolicy: Never
EOF
)

# wait for the pod to start
while true; do
    sleep 3
    PHASE=$(oc get pod -n "${E2E_TEST_NAMESPACE}" "${POD_NAME}" \
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

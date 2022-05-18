#!/bin/bash
#
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export TMP_CI_USER="appstudioci"
export CONTEXT=$(oc config current-context)
export OC_CLUSTER=$(oc config view -ojsonpath="{.contexts[?(@.name == \"$CONTEXT\")].context.cluster}")
export API_SERVER=$(oc config view -ojsonpath="{.clusters[?(@.name == \"$OC_CLUSTER\")].cluster.server}")

cat <<EOF | oc apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: htpass-secret
  namespace: openshift-config
data: 
  htpasswd: YXBwc3R1ZGlvY2k6JDJ5JDA1JEF3M0k4TFIyemVROG8yazBrb1d2dXVDSmRwL3F5ZkJLdnp0cks4MFZveEpiZFJvQlAxYy51
EOF

oc patch oauths cluster --type merge -p '
spec:
  identityProviders:
    - name: htpasswd
      mappingMethod: claim
      type: HTPasswd
      htpasswd:
        fileData:
          name: htpass-secret
'

oc adm policy add-cluster-role-to-user cluster-admin appstudioci

CURRENT_TIME=$(date +%s)
ENDTIME=$(($CURRENT_TIME + 700))
while [ $(date +%s) -lt $ENDTIME ]; do
    if oc login --kubeconfig=/tmp/kubeconfig --server $API_SERVER --username="${TMP_CI_USER}" --password=${TMP_CI_USER} --insecure-skip-tls-verify; then
        break
    fi
    sleep 10
done

export KUBECONFIG="/tmp/kubeconfig"

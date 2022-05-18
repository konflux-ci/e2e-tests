export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}

oc whoami

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
function waitToStartAuthProgress() {
    while [ $(oc get co authentication -o jsonpath="{.status.conditions[?(@.type=='Progressing')].status}") != "True" ]; do echo "Condition (status != true) failed. Waiting 2sec."; sleep 5; done
}

function waitToFinishAuthProgress() {
    while [ $(oc get co authentication -o jsonpath="{.status.conditions[?(@.type=='Progressing')].status}") != "False" ]; do echo "Condition (status != false) failed. Waiting 2sec."; sleep 20; done
}

export -f waitToStartAuthProgress
export -f waitToFinishAuthProgress

timeout --foreground 2m bash -c waitToStartAuthProgress
timeout --foreground 5m bash -c waitToFinishAuthProgress

oc adm policy add-cluster-role-to-user cluster-admin appstudioci

echo -e "[INFO] Waiting for htpasswd auth to be working up to 5 minutes"
CURRENT_TIME=$(date +%s)
ENDTIME=$(($CURRENT_TIME + 300))
while [ $(date +%s) -lt $ENDTIME ]; do
    if oc login -u appstudioci -p appstudioci --insecure-skip-tls-verify; then
        break
    fi
    sleep 10
done

oc whoami --show-token

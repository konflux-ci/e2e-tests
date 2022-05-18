export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}

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
#timeout 120 bash -x -c -- "while [[ $(oc get co authentication -o jsonpath='{.status.conditions[?(@.type=="Progressing")].status}') != False ]]; do echo 'Condition (status != False) failed. Waiting 5 secs.'; sleep 5; done"

#timeout 180s bash -x -c -- "while [[ $(oc get co authentication -o jsonpath='{.status.conditions[?(@.type=="Progressing")].status}') != True ]]; do echo 'Condition (status != true) failed. Waiting 2sec.'; sleep 5; done"

oc adm policy add-cluster-role-to-user cluster-admin appstudioci

echo -e "[INFO] Waiting for htpasswd auth to be working up to 5 minutes"
CURRENT_TIME=$(date +%s)
ENDTIME=$(($CURRENT_TIME + 300))

ctx=$(oc config current-context)
cluster=$(oc config view -ojsonpath="{.contexts[?(@.name == \"$ctx\")].context.cluster}")
server=$(oc config view -ojsonpath="{.clusters[?(@.name == \"$cluster\")].cluster.server}")
echo "Login against: $server"

export name="appstudioci"
cp "${KUBECONFIG}" "$name.kubeconfig"
occmd="bash -c '! oc login --kubeconfig=${name}.kubeconfig --username=${name} --password=appstudioci > /dev/null'"
timeout 180 "${occmd}"


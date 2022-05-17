export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}

oc whoami

cat <<EOF | oc apply -f -
apiVersion: v1
kind: Secret
metadata:
  creationTimestamp: null
  name: htpass-secret
  namespace: openshift-config
data: 
  htpasswd: YXBwc3R1ZGlvOiQyeSQwNSREY3pLblNydExBZzF0SGhWZHpTczhPWUFURFViU1NkL2wuTWRQTDFIZWtjYWtTTE1CWTFCRw==
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
echo "Waiting for authentication cluster operator to get in state Progressing==True"
timeout 60s bash -x -c -- while [ $(oc get co authentication -o jsonpath="{.status.conditions[?(@.type=="Progressing")].status}") != "True" ]; do echo "Condition (status != true) failed. Waiting 2sec."; sleep 5; done

echo "Waiting for authentication cluster operator to get in state Progressing==False"
timeout 600s bash -x -c -- while [ $(oc get co authentication -o jsonpath="{.status.conditions[?(@.type=="Progressing")].status}") != "False" ]; do echo "Condition (status != false) failed. Waiting 2sec."; sleep 20; done

oc adm policy add-cluster-role-to-user cluster-admin appstudio

echo -e "[INFO] Waiting for htpasswd auth to be working up to 5 minutes"
CURRENT_TIME=$(date +%s)
ENDTIME=$(($CURRENT_TIME + 300))
while [ $(date +%s) -lt $ENDTIME ]; do
    if oc login -u appstudio -p appstudio; then
        break
    fi
    sleep 10
done

oc whoami --show-token

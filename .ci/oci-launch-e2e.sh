export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}

touch htpasswd

htpasswd -Bb htpasswd appstudioci appstudioci

oc --user=admin create secret generic htpasswd \
    --from-file=htpasswd \ 
    -n openshift-config

oc replace -f - <<API
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
  name: cluster
spec:
  identityProviders:
  - name: Local Password
    mappingMethod: claim
    type: HTPasswd
    htpasswd:
      fileData:
        name: htpasswd
API

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

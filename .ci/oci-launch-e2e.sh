export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}

oc create secret generic htpass-secret --from-file=htpasswd="${WORKSPACE}"/scripts/resources/users.htpasswd -n openshift-config
oc apply -f "${WORKSPACE}"/scripts/resources/htpasswdProvider.yaml
oc adm policy add-cluster-role-to-user cluster-admin user

echo -e "[INFO] Waiting for htpasswd auth to be working up to 5 minutes"
CURRENT_TIME=$(date +%s)
ENDTIME=$(($CURRENT_TIME + 300))
while [ $(date +%s) -lt $ENDTIME ]; do
    if oc login -u user -p user --insecure-skip-tls-verify=false; then
        break
    fi
    sleep 10
done

oc whoami --show-token

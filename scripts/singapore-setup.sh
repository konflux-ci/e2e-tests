# use the defined below version tag
# versions greater than 2.5.0 will automatically install MCE
SNAPSHOT_VER=2.5.0-SNAPSHOT-2022-05-23-16-19-49

MY_NAMESPACE="cluster-reg-config"
HUB_NAME="mcehub"
SECRET_NAME="hub-kubeconfig"

export DOCKERCONFIGJSON=$(cat /usr/local/ci-secrets/redhat-appstudio-qe/stolostron_e2e)

# inherit current workspace or set a new one if not exists
if [[ -z "${WORKSPACE}" ]]; then
  ROOT_WKS=$PWD
  echo "WORKSPACE is not set. Using $ROOT_WKS"
  mkdir $ROOT_WKS/tmp
  WORKSPACE=$ROOT_WKS
fi

# deploy installs ACM to the cluster pointed by your KUBECONFIG env variable
if [[ -z "${KUBECONFIG}" ]]; then
  echo "KUBECONFIG is not set"
  exit
fi

# configure prerequisites for ACM setup and run setup script
function prepareAndInstallACM() {
    # clone the deploy repo
    git clone https://github.com/stolostron/deploy.git "$WORKSPACE"/tmp/deploy

    # put pull-secret.yaml in prereqs folder
    echo "apiVersion: v1
kind: Secret
metadata:
  name: multiclusterhub-operator-pull-secret
data:
  .dockerconfigjson: $DOCKERCONFIGJSON
type: kubernetes.io/dockerconfigjson" > "$WORKSPACE"/tmp/deploy/prereqs/pull-secret.yaml

    # configure snapshot.ver with the SNAPSHOT tag to install in silent mode
    echo "$SNAPSHOT_VER" > "$WORKSPACE"/tmp/deploy/snapshot.ver
    echo "snapshot.ver set to $SNAPSHOT_VER"

    # Install Open Cluster Management
    cd "$WORKSPACE"/tmp/deploy 
    ./start.sh --silent --search --watch
    cd ..
}

# create the secret needed to onboard a managed hub cluster
function onboardManagedHubCluster() {
    # ensure that on the managed hub, the multiclusterengine CR has the managedserviceaccount-preview enabled
    oc patch multiclusterengine multiclusterengine --type=merge -p '{"spec":{"overrides":{"components":[{"name":"managedserviceaccount-preview","enabled":true}]}}}'
    echo "multiclusterengine CR patched to enable managedserviceaccount-preview"

    # get the kubeconfig of the managed hub cluster and 
    # create the secret in the appstudio cluster using the managed hub cluster kubeconfig 
    # NOTE: this script deploys MCE and AppStudio on the same cluster
    oc create secret generic $SECRET_NAME --from-file=kubeconfig=$KUBECONFIG -n $MY_NAMESPACE
    echo "created secret $SECRET_NAME from kubeconfig file"
}

function startClusterRegistrationController() {
    # create the hub config on the AppStudio cluster
    echo "
apiVersion: singapore.open-cluster-management.io/v1alpha1
kind: HubConfig
metadata:
  name: $HUB_NAME
  namespace: $MY_NAMESPACE
spec:
  kubeConfigSecretRef:
    name: $SECRET_NAME
" | oc create -f -

    # create the clusterregistrar on the AppStudio cluster  
    echo "
apiVersion: singapore.open-cluster-management.io/v1alpha1
kind: ClusterRegistrar
metadata:
  name: cluster-reg
spec:" | oc create -f -

    # verify pods are running
}

prepareAndInstallACM
onboardManagedHubCluster
startClusterRegistrationController


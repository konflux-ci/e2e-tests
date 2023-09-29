#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# Login to the stage member cluster with the OCP_TOKEN credentials
TOKEN=$(echo ${OCP_TOKEN} | base64 -d)
oc login --token="$TOKEN" --server="$STAGE_MEMBER_CLUSTER"


ARTIFACT_DIR=${ARTIFACT_DIR:-.artifacts}
mkdir -p ${ARTIFACT_DIR}
pushd "${1:-./tests/load-tests}"

echo "Collecting load test results"
cp -vf *.log "${ARTIFACT_DIR}"
cp -vf load-tests.json "${ARTIFACT_DIR}"

monitoring_collection_log=$ARTIFACT_DIR/monitoring-collection.log
monitoring_collection_data=$ARTIFACT_DIR/load-tests.json

## Monitoring data
echo "Setting up tool to collect monitoring data..."
python3 -m venv venv
set +u
source venv/bin/activate
set -u
python3 -m pip install -U pip
python3 -m pip install -e "git+https://github.com/redhat-performance/opl.git#egg=opl-rhcloud-perf-team-core&subdirectory=core"

echo "Collecting monitoring data..."
mstart=$(date --utc --date "$(status_data.py --status-data-file "$monitoring_collection_data" --get timestamp)" --iso-8601=seconds)
mend=$(date --utc --date "$(status_data.py --status-data-file "$monitoring_collection_data" --get endTimestamp)" --iso-8601=seconds)
mhost=$(oc -n openshift-monitoring get route -l app.kubernetes.io/name=thanos-query -o json | jq --raw-output '.items[0].spec.host')

status_data.py \
    --status-data-file "$monitoring_collection_data" \
    --additional ./cluster_read_config.yaml \
    --monitoring-start "$mstart" \
    --monitoring-end "$mend" \
    --prometheus-host "https://$mhost" \
    --prometheus-port 443 \
    --prometheus-token "$TOKEN" \
    -d &>$monitoring_collection_log

if [ $? -ne 0 ]; then
    echo "Error: status_data.py failed with exit code $?"
fi    

set +u
deactivate
set -u

popd

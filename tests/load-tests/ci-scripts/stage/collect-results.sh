#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

source "$( dirname $0 )/../utils.sh"

echo "[$(date --utc -Ins)] Collecting load test results"

# Setup directories
ARTIFACT_DIR=${ARTIFACT_DIR:-artifacts}
mkdir -p ${ARTIFACT_DIR}
pushd "${2:-./tests/load-tests}"

# Construct $PROMETHEUS_HOST by extracting BASE_URL from $MEMBER_CLUSTER
BASE_URL=$(echo $MEMBER_CLUSTER | grep -oP 'https://api\.\K[^:]+')
PROMETHEUS_HOST="thanos-querier-openshift-monitoring.apps.$BASE_URL"
TOKEN=${OCP_PROMETHEUS_TOKEN}

echo "[$(date --utc -Ins)] Collecting artifacts"
find . -maxdepth 1 -type f -name '*.log' -exec cp -vf {} "${ARTIFACT_DIR}" \;
find . -maxdepth 1 -type f -name '*.csv' -exec cp -vf {} "${ARTIFACT_DIR}" \;
find . -maxdepth 1 -type f -name 'load-test-options.json' -exec cp -vf {} "${ARTIFACT_DIR}" \;
find . -maxdepth 1 -type d -name 'collected-data' -exec cp -r {} "${ARTIFACT_DIR}" \;

echo "[$(date --utc -Ins)] Setting up Python venv"
{
python3 -m venv venv
set +u
source venv/bin/activate
set -u
python3 -m pip install -U pip
python3 -m pip install -e "git+https://github.com/redhat-performance/opl.git#egg=opl-rhcloud-perf-team-core&subdirectory=core"
python3 -m pip install tabulate
python3 -m pip install matplotlib
} &>"${ARTIFACT_DIR}/monitoring-setup.log"

echo "[$(date --utc -Ins)] Create summary JSON with timings"
./evaluate.py "${ARTIFACT_DIR}/load-test-timings.csv" "${ARTIFACT_DIR}/load-test-timings.json"

echo "[$(date --utc -Ins)] Create summary JSON with errors"
./errors.py "${ARTIFACT_DIR}/load-test-errors.csv" "${ARTIFACT_DIR}/load-test-errors.json"

echo "[$(date --utc -Ins)] Graphing PRs and TRs"
ci-scripts/utility_scripts/show-pipelineruns.py --data-dir "${ARTIFACT_DIR}" &>"${ARTIFACT_DIR}/show-pipelineruns.log" || true
mv "${ARTIFACT_DIR}/output.svg" "${ARTIFACT_DIR}/show-pipelines.svg" || true

echo "[$(date --utc -Ins)] Computing duration of PRs, TRs and steps"
ci-scripts/utility_scripts/get-taskruns-durations.py --debug --data-dir "${ARTIFACT_DIR}" --dump-json "${ARTIFACT_DIR}/get-taskruns-durations.json" &>"${ARTIFACT_DIR}/get-taskruns-durations.log"

echo "[$(date --utc -Ins)] Creating main status data file"
STATUS_DATA_FILE="${ARTIFACT_DIR}/load-test.json"
status_data.py \
    --status-data-file "${STATUS_DATA_FILE}" \
    --set "name=Konflux loadtest" "started=$( cat started )" "ended=$( cat ended )" \
    --set-subtree-json "parameters.options=${ARTIFACT_DIR}/load-test-options.json" "results.measurements=${ARTIFACT_DIR}/load-test-timings.json" "results.errors=${ARTIFACT_DIR}/load-test-errors.json" "results.durations=${ARTIFACT_DIR}/get-taskruns-durations.json"

echo "[$(date --utc -Ins)] Adding monitoring data"
mstarted="$( date -d "$( cat started )" --utc -Iseconds )"
mended="$( date -d "$( cat ended )" --utc -Iseconds )"
mhost="https://$PROMETHEUS_HOST"
mrawdir="${ARTIFACT_DIR}/monitoring-raw-data-dir/"
mkdir -p "$mrawdir"
status_data.py \
    --status-data-file "${STATUS_DATA_FILE}" \
    --additional ci-scripts/stage/cluster_read_config.yaml \
    --monitoring-start "$mstarted" \
    --monitoring-end "$mended" \
    --prometheus-host "$mhost" \
    --prometheus-port 443 \
    --prometheus-token "$TOKEN" \
    --monitoring-raw-data-dir "$mrawdir" \
    &>"${ARTIFACT_DIR}/monitoring-collection.log"

deactivate

popd

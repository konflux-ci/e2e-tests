#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# shellcheck disable=SC1090
source "/usr/local/ci-secrets/redhat-appstudio-load-test/load-test-scenario.${1:-concurrent}"

pushd "${2:-.}"

echo "Collecting load test results"
cp -f ./tests/load-tests/load-tests.max-concurrency.*.log "$ARTIFACT_DIR"

echo "Setting up tool to collect monitoring data..."
python3 -m venv venv
set +u
source venv/bin/activate
set -u
python3 -m pip install -U pip
python3 -m pip install -U pip
python3 -m pip install -e "git+https://github.com/redhat-performance/opl.git#egg=opl-rhcloud-perf-team-core&subdirectory=core"

for monitoring_collection_data in ./tests/load-tests/load-tests.max-concurrency.*.json; do
    cp -f "$monitoring_collection_data" "$ARTIFACT_DIR"
    index=$(echo "$monitoring_collection_data" | sed -e 's,.*/load-tests.max-concurrency.\([0-9]\+\).json,\1,')
    monitoring_collection_log="$ARTIFACT_DIR/monitoring-collection.$index.log"

    ## Monitoring data
    echo "Collecting monitoring data for step $index..."
    mstart=$(date --utc --date "$(status_data.py --status-data-file "$monitoring_collection_data" --get timestamp)" --iso-8601=seconds)
    mend=$(date --utc --date "$(status_data.py --status-data-file "$monitoring_collection_data" --get endTimestamp)" --iso-8601=seconds)
    mhost=$(oc -n openshift-monitoring get route -l app.kubernetes.io/name=thanos-query -o json | jq --raw-output '.items[0].spec.host')
    status_data.py \
        --status-data-file "$monitoring_collection_data" \
        --additional ./tests/load-tests/cluster_read_config.yaml \
        --monitoring-start "$mstart" \
        --monitoring-end "$mend" \
        --prometheus-host "https://$mhost" \
        --prometheus-port 443 \
        --prometheus-token "$(oc whoami -t)" \
        -d &>$monitoring_collection_log
done
set +u
deactivate
set -u

csv_delim=";"
csv_delim_quoted="\"$csv_delim\""
dt_format='"%Y-%m-%dT%H:%M:%SZ"'

## PipelineRun timestamps
echo "Collecting PipelineRun timestamps..."
pipelinerun_timestamps=$ARTIFACT_DIR/pipelineruns.tekton.dev_timestamps.csv
echo "PipelineRun${csv_delim}Namespace${csv_delim}Succeeded${csv_delim}Reason${csv_delim}Message${csv_delim}Created${csv_delim}Started${csv_delim}FinallyStarted${csv_delim}Completed${csv_delim}Created->Started${csv_delim}Started->FinallyStarted${csv_delim}FinallyStarted->Completed${csv_delim}SucceededDuration${csv_delim}FailedDuration" >"$pipelinerun_timestamps"
jq_cmd=".items[] | (.metadata.name) \
+ $csv_delim_quoted + (.metadata.namespace) \
+ $csv_delim_quoted + (.status.conditions[0].status) \
+ $csv_delim_quoted + (.status.conditions[0].reason) \
+ $csv_delim_quoted + (.status.conditions[0].message|split($csv_delim_quoted)|join(\"_\")) \
+ $csv_delim_quoted + (.metadata.creationTimestamp) \
+ $csv_delim_quoted + (.status.startTime) \
+ $csv_delim_quoted + (.status.finallyStartTime) \
+ $csv_delim_quoted + (.status.completionTime) \
+ $csv_delim_quoted + (if .status.startTime != null and .metadata.creationTimestamp != null then ((.status.startTime | strptime($dt_format) | mktime) - (.metadata.creationTimestamp | strptime($dt_format) | mktime) | tostring) else \"\" end) \
+ $csv_delim_quoted + (if .status.finallyStartTime != null and .status.startTime != null then ((.status.finallyStartTime | strptime($dt_format) | mktime) - (.status.startTime | strptime($dt_format) | mktime) | tostring) else \"\" end) \
+ $csv_delim_quoted + (if .status.completionTime != null and .status.finallyStartTime != null then ((.status.completionTime | strptime($dt_format) | mktime) - (.status.finallyStartTime | strptime($dt_format) | mktime) | tostring) else \"\" end) \
+ $csv_delim_quoted + (if .status.conditions[0].status == \"True\" and .status.completionTime != null and .metadata.creationTimestamp != null then ((.status.completionTime | strptime($dt_format) | mktime) - (.metadata.creationTimestamp | strptime($dt_format) | mktime) | tostring) else \"\" end) \
+ $csv_delim_quoted + (if .status.conditions[0].status == \"False\" and .status.completionTime != null and .metadata.creationTimestamp != null then ((.status.completionTime | strptime($dt_format) | mktime) - (.metadata.creationTimestamp | strptime($dt_format) | mktime) | tostring) else \"\" end)"
oc get pipelineruns.tekton.dev -A -o json | jq "$jq_cmd" | sed -e "s/\n//g" -e "s/^\"//g" -e "s/\"$//g" -e "s/Z;/;/g" >>"$pipelinerun_timestamps"

popd

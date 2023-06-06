#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# shellcheck disable=SC1090
source "/usr/local/ci-secrets/redhat-appstudio-load-test/load-test-scenario.${1:-concurrent}"

pushd "${2:-.}"

echo "Collecting load test results"
cp -vf ./tests/load-tests/load-tests.max-concurrency.*.log "$ARTIFACT_DIR"
cp -vf ./tests/load-tests/load-tests.max-concurrency.json "$ARTIFACT_DIR"

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

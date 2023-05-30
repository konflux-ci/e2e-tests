#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# shellcheck disable=SC1090
source "/usr/local/ci-secrets/redhat-appstudio-load-test/load-test-scenario.${1:-concurrent}"

pushd "${2:-.}"

echo "Collecting load test results"
load_test_log=$ARTIFACT_DIR/load-tests.log
cp -vf ./tests/load-tests/load-tests.log "$load_test_log"
cp -vf ./tests/load-tests/load-tests.json "$ARTIFACT_DIR"
cp -vf ./tests/load-tests/gh-rate-limits-remaining.csv "$ARTIFACT_DIR"

application_timestamps=$ARTIFACT_DIR/applications.appstudio.redhat.com_timestamps
application_timestamps_csv=${application_timestamps}.csv
application_timestamps_txt=${application_timestamps}.txt
componentdetectionquery_timestamps=$ARTIFACT_DIR/componentdetectionqueries.appstudio.redhat.com_timestamps.csv
component_timestamps=$ARTIFACT_DIR/components.appstudio.redhat.com_timestamps.csv
pipelinerun_timestamps=$ARTIFACT_DIR/pipelineruns.tekton.dev_timestamps.csv
application_service_log=$ARTIFACT_DIR/application-service.log
application_service_log_segments=$ARTIFACT_DIR/application-service-log-segments
csv_delim=";"
csv_delim_quoted="\"$csv_delim\""
dt_format='"%Y-%m-%dT%H:%M:%SZ"'
## Application timestamps
echo "Collecting Application timestamps..."
echo "Application${csv_delim}StatusSucceeded${csv_delim}StatusMessage${csv_delim}CreatedTimestamp${csv_delim}SucceededTimestamp${csv_delim}Duration" >"$application_timestamps_csv"
jq_cmd=".items[] | (.metadata.name) \
+ $csv_delim_quoted + (.status.conditions[0].status) \
+ $csv_delim_quoted + (.status.conditions[0].message) \
+ $csv_delim_quoted + (.metadata.creationTimestamp) \
+ $csv_delim_quoted + (.status.conditions[0].lastTransitionTime) \
+ $csv_delim_quoted + ((.status.conditions[0].lastTransitionTime | strptime($dt_format) | mktime) - (.metadata.creationTimestamp | strptime($dt_format) | mktime) | tostring)"
oc get applications.appstudio.redhat.com -A -o json | jq -rc "$jq_cmd" | sed -e 's,Z,,g' >>"$application_timestamps_csv"
oc get applications.appstudio.redhat.com -A -o 'custom-columns=NAME:.metadata.name,CREATED:.metadata.creationTimestamp,LAST_UPDATED:.status.conditions[0].lastTransitionTime,STATUS:.status.conditions[0].reason,MESSAGE:.status.conditions[0].message' >"$application_timestamps_txt"

## ComponentDetectionQuery timestamps
echo "Collecting ComponentDetectionQuery timestamps..."
echo "ComponentDetectionQuery${csv_delim}Namespace${csv_delim}CreationTimestamp${csv_delim}Completed${csv_delim}Completed.Reason${csv_delim}Completed.Mesasge${csv_delim}Duration" >"$componentdetectionquery_timestamps"
jq_cmd=".items[] | (.metadata.name) \
+ $csv_delim_quoted + (.metadata.namespace) \
+ $csv_delim_quoted + (.metadata.creationTimestamp) \
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"Completed\")) // false) then (.status.conditions[] | select(.type == \"Completed\") | .lastTransitionTime + $csv_delim_quoted + .reason + $csv_delim_quoted + .message) else \"$csv_delim$csv_delim\" end)\
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"Completed\")) // false) then ((.status.conditions[] | select(.type == \"Completed\") | .lastTransitionTime | strptime($dt_format) | mktime) - (.metadata.creationTimestamp | strptime($dt_format) | mktime) | tostring) else \"\" end)"
oc get componentdetectionqueries.appstudio.redhat.com -A -o json | jq -rc "$jq_cmd" | sed -e 's,Z,,g' >>"$componentdetectionquery_timestamps"

## Component timestamps
echo "Collecting Component timestamps..."
echo "Component${csv_delim}Namespace${csv_delim}CreationTimestamp${csv_delim}Created${csv_delim}Created.Reason${csv_delim}Create.Mesasge${csv_delim}GitOpsResourcesGenerated${csv_delim}GitOpsResourcesGenerated.Reason${csv_delim}GitOpsResourcesGenerated.Message${csv_delim}Updated${csv_delim}Updated.Reason${csv_delim}Updated.Message${csv_delim}CreationTimestamp->Created${csv_delim}Created->GitOpsResourcesGenerated${csv_delim}GitOpsResourcesGenerated->Updated${csv_delim}Duration" >"$component_timestamps"
jq_cmd=".items[] | (.metadata.name) \
+ $csv_delim_quoted + (.metadata.namespace) \
+ $csv_delim_quoted + (.metadata.creationTimestamp) \
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"Created\")) // false) then (.status.conditions[] | select(.type == \"Created\") | .lastTransitionTime + $csv_delim_quoted + .reason + $csv_delim_quoted + (.message|split($csv_delim_quoted)|join(\"_\"))) else \"$csv_delim$csv_delim\" end) \
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"GitOpsResourcesGenerated\")) // false) then (.status.conditions[] | select(.type == \"GitOpsResourcesGenerated\") | .lastTransitionTime + $csv_delim_quoted + .reason + $csv_delim_quoted + (.message|split($csv_delim_quoted)|join(\"_\"))) else \"$csv_delim$csv_delim\" end) \
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"Updated\")) // false) then (.status.conditions[] | select(.type == \"Updated\") | .lastTransitionTime + $csv_delim_quoted + .reason + $csv_delim_quoted + (.message|split($csv_delim_quoted)|join(\"_\"))) else \"$csv_delim$csv_delim\" end) \
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"Created\")) // false) then ((.status.conditions[] | select(.type == \"Created\") | .lastTransitionTime | strptime($dt_format) | mktime) - (.metadata.creationTimestamp | strptime($dt_format) | mktime) | tostring) else \"\" end)\
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"GitOpsResourcesGenerated\")) // false) and ((.status.conditions[] | select(.type == \"Created\")) // false) then ((.status.conditions[] | select(.type == \"GitOpsResourcesGenerated\") | .lastTransitionTime | strptime($dt_format) | mktime) - (.status.conditions[] | select(.type == \"Created\") | .lastTransitionTime | strptime($dt_format) | mktime) | tostring) else \"\" end) \
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"Updated\")) // false) and ((.status.conditions[] | select(.type == \"GitOpsResourcesGenerated\")) // false) then ((.status.conditions[] | select(.type == \"Updated\") | .lastTransitionTime | strptime($dt_format) | mktime) - (.status.conditions[] | select(.type == \"GitOpsResourcesGenerated\") | .lastTransitionTime | strptime($dt_format) | mktime) | tostring) else \"\" end) \
+ $csv_delim_quoted + (if ((.status.conditions[] | select(.type == \"Updated\")) // false) then ((.status.conditions[] | select(.type == \"Updated\") | .lastTransitionTime | strptime($dt_format) | mktime) - (.metadata.creationTimestamp | strptime($dt_format) | mktime) | tostring) else \"\" end)"
oc get components.appstudio.redhat.com -A -o json | jq -rc "$jq_cmd" | sed -e 's,Z,,g' >>"$component_timestamps"

## PipelineRun timestamps
echo "Collecting PipelineRun timestamps..."
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
oc get pipelineruns.tekton.dev -A -o json | jq "$jq_cmd" | sed -e "s/\n//g" -e "s/^\"//g" -e "s/\"$//g" -e "s/Z;/;/g" >>$pipelinerun_timestamps

## Application service log segments per user app
echo "Collecting application service log segments per user app..."
oc logs -l "control-plane=controller-manager" --tail=-1 -n application-service >"$application_service_log"
mkdir -p "$application_service_log_segments"
for i in $(grep -Eo "${USER_PREFIX}-....-app" "$application_service_log" | sort | uniq); do grep "$i" "$application_service_log" >"$application_service_log_segments/$i.log"; done
## Error summary
echo "Error summary:"
if [ -f "$load_test_log" ]; then
    grep -Eo "Error #[0-9]+" "$load_test_log" | sort | uniq | while read -r i; do
        echo -n " - $i: "
        grep -c "$i" "$load_test_log"
    done | sort -V || :
else
    echo "File $load_test_log does not exist!"
    exit 1
fi
popd

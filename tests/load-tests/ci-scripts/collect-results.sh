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
## Application timestamps
echo "Collecting Application timestamps..."
echo "Application;StatusSucceeded;StatusMessage;CreatedTimestamp;SucceededTimestamp" >"$application_timestamps_csv"
oc get applications.appstudio.redhat.com -A -o json | jq -rc '.items[] | (.metadata.name) + ";" + (.status.conditions[0].status) + ";" + (.status.conditions[0].message) + ";" + (.metadata.creationTimestamp) + ";" + (.status.conditions[0].lastTransitionTime)' | sed -e 's,Z,,g' >>"$application_timestamps_csv"
oc get applications.appstudio.redhat.com -A -o 'custom-columns=NAME:.metadata.name,CREATED:.metadata.creationTimestamp,LAST_UPDATED:.status.conditions[0].lastTransitionTime,STATUS:.status.conditions[0].reason,MESSAGE:.status.conditions[0].message' >"$application_timestamps_txt"
## ComponentDetectionQuery timestamps
echo "Collecting ComponentDetectionQuery timestamps..."
echo "ComponentDetectionQuery;Namespace;CreationTimestamp;Completed;Completed.Reason;Completed.Mesasge" >"$componentdetectionquery_timestamps"
oc get componentdetectionqueries.appstudio.redhat.com -A -o json | jq -rc '.items[] | (.metadata.name) + ";" + (.metadata.namespace) + ";" + (.metadata.creationTimestamp) + ";" + (.status.conditions[] | select(.type == "Completed") | .lastTransitionTime + ";" + .reason + ";" + .message + "")' | sed -e 's,Z,,g' >>"$componentdetectionquery_timestamps"
## Component timestamps
echo "Collecting Component timestamps..."
echo "Component;Namespace;CreationTimestamp;Created;Created.Reason;Create.Mesasge;GitOpsResourcesGenerated;GitOpsResourcesGenerated.Reason;GitOpsResourcesGenerated.Message;Updated;Updated.Reason;Updated.Message" >"$component_timestamps"
oc get components.appstudio.redhat.com -A -o json | jq -rc '.items[] | (.metadata.name) + ";" + (.metadata.namespace) + ";" + (.metadata.creationTimestamp) + ";" + (.status.conditions[] | select(.type == "Created") | .lastTransitionTime + ";" + .reason + ";" + (.message|split(";")|join("_"))) + ";" + (.status.conditions[] | select(.type == "GitOpsResourcesGenerated") | .lastTransitionTime + ";" + .reason + ";" + (.message|split(";")|join("_"))) + ";" + (.status.conditions[] | select(.type == "Updated") | .lastTransitionTime + ";" + .reason + ";" + (.message|split(";")|join("_")))' | sed -e 's,Z,,g' >>"$component_timestamps"
## PipelineRun timestamps
echo "Collecting PipelineRun timestamps..."
echo "PipelineRun;Namespace;Succeeded;Reason;Message;Created;Started;FinallyStarted;Completed" >"$pipelinerun_timestamps"
oc get pipelineruns.tekton.dev -A -o json | jq -r '.items[] | (.metadata.name) + ";" + (.metadata.namespace) + ";" + (.status.conditions[0].status) + ";" + (.status.conditions[0].reason) + ";" + (.status.conditions[0].message) + ";"  + (.metadata.creationTimestamp) + ";" + (.status.startTime) + ";" + (.status.finallyStartTime) + ";" + (.status.completionTime)' >>"$pipelinerun_timestamps"
## Application service log segments per user app
echo "Collecting application service log segments per user app..."
oc logs -l "control-plane=controller-manager" --tail=-1 -n application-service >"$application_service_log"
mkdir -p "$application_service_log_segments"
for i in $(grep -Eo "${USER_PREFIX}-....-app" "$application_service_log" | sort | uniq); do grep "$i" "$application_service_log" >"$application_service_log_segments/$i.log"; done
## Error summary
echo "Number of errors occurred in load test log:"
grep -Eo "Error #[0-9]+" "$load_test_log" | sort | uniq | while read -r i; do
    echo -n " - $i: "
    grep -c "$i" "$load_test_log"
done | sort -V

popd

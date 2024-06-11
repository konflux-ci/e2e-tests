#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail


csv_delim=";"
csv_delim_quoted="\"$csv_delim\""
dt_format='"%Y-%m-%dT%H:%M:%SZ"'


function collect_application() {
    local oc_opts="${1:--A}"
    local file_stub="${2:-$ARTIFACT_DIR/collected-applications.appstudio.redhat.com}"
    local file_csv="${file_stub}.csv"
    local file_json="${file_stub}.json"

    oc get applications.appstudio.redhat.com $oc_opts -o json >"$file_json"

    echo "Application${csv_delim}CreatedTimestamp" >"$file_csv"
    local jq_cmd=".items[] | (.metadata.name) \
    + $csv_delim_quoted + (.metadata.creationTimestamp)"
    cat "$file_json" | jq -rc "$jq_cmd" | sed -e 's,Z,,g' >>"$file_csv"
}

function collect_component() {
    local oc_opts="${1:--A}"
    local file_stub="${2:-$ARTIFACT_DIR/collected-components.appstudio.redhat.com}"
    local file_csv="${file_stub}.csv"
    local file_json="${file_stub}.json"

    oc get components.appstudio.redhat.com $oc_opts -o json >"$file_json"

    echo "Component${csv_delim}Namespace${csv_delim}CreationTimestamp" >"$file_csv"
    jq_cmd=".items[] | (.metadata.name) \
    + $csv_delim_quoted + (.metadata.namespace) \
    + $csv_delim_quoted + (.metadata.creationTimestamp)"
    cat "$file_json" | jq -rc "$jq_cmd" | sed -e 's,Z,,g' >>"$file_csv"
}

function collect_pipelinerun() {
    local oc_opts="${1:--A}"
    local file_stub="${2:-$ARTIFACT_DIR/collected-pipelineruns.tekton.dev}"
    local file_csv="${file_stub}.csv"
    local file_json="${file_stub}.json"

    oc get pipelineruns.tekton.dev $oc_opts -o json >"$file_json"

    echo "PipelineRun${csv_delim}Namespace${csv_delim}Succeeded${csv_delim}Reason${csv_delim}Message${csv_delim}Created${csv_delim}Started${csv_delim}FinallyStarted${csv_delim}Completed${csv_delim}Created->Started${csv_delim}Started->FinallyStarted${csv_delim}FinallyStarted->Completed${csv_delim}SucceededDuration${csv_delim}FailedDuration" >"$file_csv"
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
    cat "$file_json" | jq "$jq_cmd" | sed -e "s/\n//g" -e "s/^\"//g" -e "s/\"$//g" -e "s/Z;/;/g" | sort -t ";" -k 13 -r -n >>"$file_csv"
}

function collect_taskrun() {
    local oc_opts="${1:--A}"
    local file_stub="${2:-$ARTIFACT_DIR/collected-taskruns.tekton.dev}"
    local file_json="${file_stub}.json"

    oc get taskruns.tekton.dev $oc_opts -o json >"$file_json"
}

function collect_pods() {
    local oc_opts="${1:--A}"
    local file_stub="${2:-$ARTIFACT_DIR/collected-pods}"
    local file_json="${file_stub}.json"
    local file_logs="${file_stub}.log"

    oc get pod $oc_opts -o json >"$file_json"

    pods_on_nodes_csv="${file_stub}-on-nodes.csv"
    echo "Node;Namespace;Pod" >"$pods_on_nodes_csv"
    jq_cmd=".items[] | select(.metadata.labels.\"appstudio.openshift.io/application\" != null) \
    | .spec.nodeName \
    + $csv_delim_quoted + .metadata.namespace \
    + $csv_delim_quoted + .metadata.name"
    cat "$file_json" | jq -r "$jq_cmd" | sort -V >>"$pods_on_nodes_csv"

    all_pods_distribution_csv="${file_stub}-distribution.csv"
    echo "Node;Pods" >"$all_pods_distribution_csv"
    cat "$file_json" | jq -r ".items[] | .spec.nodeName" | sort | uniq -c | sed -e 's,\s\+\([0-9]\+\)\s\+\(.*\),\2;\1,g' >>"$all_pods_distribution_csv"

    task_pods_distribution_csv="${file_stub}-task-distribution.csv"
    echo "Node;Pods" >"$task_pods_distribution_csv"
    cat "$file_json" | jq -r '.items[] | select(.metadata.labels."appstudio.openshift.io/application" != null).spec.nodeName' | sort | uniq -c | sed -e 's,\s\+\([0-9]\+\)\s\+\(.*\),\2;\1,g' >>"$task_pods_distribution_csv"

    oc get pod $oc_opts -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name --no-headers=true | while IFS=$'\n' read row; do
        ns=$( echo "$row" | sed 's/\s\+.*$//' )
        name=$( echo "$row" | sed 's/^.*\s\+//' )
        echo -e "\n\n##### $ns - $name #####\n\n" >>"$file_logs"
        oc -n "$ns" logs --prefix=true --all-containers=true --timestamps=true "$name" >>"$file_logs"
    done
}

function collect_nodes() {
    local file_stub="${1:-$ARTIFACT_DIR/collected-nodes}"
    local file_csv="${file_stub}.csv"
    local file_json="${file_stub}.json"

    oc get nodes -o json >"$file_json"

    echo "Node;CPUs;Memory;InstanceType;NodeType;Zone" >"$file_csv"
    jq_cmd=".items[] | .metadata.name \
    + $csv_delim_quoted + .status.capacity.cpu \
    + $csv_delim_quoted + .status.capacity.memory \
    + $csv_delim_quoted + .metadata.labels.\"node.kubernetes.io/instance-type\" \
    + $csv_delim_quoted + (if .metadata.labels.\"node-role.kubernetes.io/worker\" != null then \"worker\" else \"master\" end) \
    + $csv_delim_quoted + .metadata.labels.\"topology.kubernetes.io/zone\""
    cat "$file_json" | jq -r "$jq_cmd" >>"$file_csv"
}

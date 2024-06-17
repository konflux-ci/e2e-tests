#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# shellcheck disable=SC1090
source "/usr/local/ci-secrets/redhat-appstudio-load-test/load-test-scenario.${1:-concurrent}"

pushd "${2:-.}"

output_dir="${OUTPUT_DIR:-./tests/load-tests}"

csv_delim=";"
csv_delim_quoted="\"$csv_delim\""
dt_format='"%Y-%m-%dT%H:%M:%SZ"'

collect_artifacts() {
    echo "Collecting load test artifacts.."
    find "$output_dir" -type f -name 'load-tests.max-concurrency.json' -exec cp -vf {} "${ARTIFACT_DIR}" \;
    mkdir -p "${ARTIFACT_DIR}/iterations"
    find "$output_dir" -maxdepth 1 -type d -name 'iteration-*' -exec cp -vfr {} "${ARTIFACT_DIR}/iterations" \;
    mkdir -p "${ARTIFACT_DIR}/pprof"
    find "$output_dir" -type f -name '*.pprof' -exec cp -vf {} "${ARTIFACT_DIR}/pprof" \;
}

collect_monitoring_data() {
    echo "Collecting monitoring data..."
    echo "Setting up tool to collect monitoring data..."
    python3 -m venv venv
    set +u
    # shellcheck disable=SC1091
    source venv/bin/activate
    set -u
    python3 -m pip install -U pip
    python3 -m pip install -e "git+https://github.com/redhat-performance/opl.git#egg=opl-rhcloud-perf-team-core&subdirectory=core"

    ## Monitoring data for entire test
    monitoring_collection_data=$(find "$output_dir" -type f -name 'load-tests.max-concurrency.json')
    monitoring_collection_log="$ARTIFACT_DIR/monitoring-collection.log"
    monitoring_collection_dir=$ARTIFACT_DIR/monitoring-collection-raw-data-dir
    mkdir -p "$monitoring_collection_dir"
    echo "Collecting monitoring data for entire test"
    mstart=$(date --utc --date "$(status_data.py --status-data-file "$monitoring_collection_data" --get startTimestamp)" --iso-8601=seconds)
    mend=$(date --utc --date "$(status_data.py --status-data-file "$monitoring_collection_data" --get endTimestamp)" --iso-8601=seconds)
    mhost=$(oc -n openshift-monitoring get route -l app.kubernetes.io/name=thanos-query -o json | jq --raw-output '.items[0].spec.host')
    status_data.py \
        --status-data-file "$monitoring_collection_data" \
        --additional ./tests/load-tests/ci-scripts/max-concurrency/cluster_read_config.yaml \
        --monitoring-start "$mstart" \
        --monitoring-end "$mend" \
        --monitoring-raw-data-dir "$monitoring_collection_dir" \
        --prometheus-host "https://$mhost" \
        --prometheus-port 443 \
        --prometheus-token "$(oc whoami -t)" \
        -d &>"$monitoring_collection_log"
    cp -f "$monitoring_collection_data" "$ARTIFACT_DIR"

    ## Monitoring data per iteration
    for monitoring_collection_data in $(find "$output_dir" -type f -name 'load-tests.max-concurrency.*.json'); do
        iteration_index=$(echo "$monitoring_collection_data" | sed -e 's,.*/load-tests.max-concurrency.\([0-9]\+-[0-9]\+\).json,\1,')
        monitoring_collection_log="$ARTIFACT_DIR/monitoring-collection.$iteration_index.log"
        echo "Collecting monitoring data for step $iteration_index..."
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
            -d &>"$monitoring_collection_log"
        cp -f "$monitoring_collection_data" "$ARTIFACT_DIR"
    done
    set +u
    deactivate
    set -u
}

collect_tekton_profiling_data() {
    echo "Collecting Tekton prifiling data..."
    if [ "${TEKTON_PERF_ENABLE_CPU_PROFILING:-}" == "true" ] || [ "${TEKTON_PERF_ENABLE_MEMORY_PROFILING:-}" == "true" ]; then
        echo "Collecting profiling data from Tekton"
        for pprof_profile in $(find "$output_dir" -name "*.pprof"); do
            if [ -s "$pprof_profile" ]; then
                file=$(basename "$pprof_profile")
                go tool pprof -text "$pprof_profile" >"$ARTIFACT_DIR/pprof/$file.txt" || true
                go tool pprof -svg -output="$ARTIFACT_DIR/pprof/$file.svg" "$pprof_profile" || true
            fi
        done
    fi
}

get_tekton_results_watcher_pod_count() {
    find "$output_dir" -type f -name 'tekton-results-watcher.tekton-results-watcher-*.goroutine-dump-0.0001-*.pprof' | wc -l
}

collect_scalability_data() {
    echo "Collecting scalability data..."

    tekton_results_watcher_pod_count=$(get_tekton_results_watcher_pod_count)
    tekton_results_watcher_pod_headers=""
    for i in $(seq -w 1 "$tekton_results_watcher_pod_count"); do
        tekton_results_watcher_pod_headers="${tekton_results_watcher_pod_headers}${csv_delim}ParkedGoRoutinesPod$i"
    done

    max_concurrency_csv=$ARTIFACT_DIR/max-concurrency.csv
    echo "Iteration\
${csv_delim}Threads\
${csv_delim}WorkloadKPI\
${csv_delim}Errors\
${csv_delim}UserAvgTime\
${csv_delim}UserMaxTime\
${csv_delim}ApplicationAvgTime\
${csv_delim}ApplicationMaxTime\
${csv_delim}CDQAvgTime\
${csv_delim}CDQMaxTime\
${csv_delim}ComponentsAvgTime\
${csv_delim}ComponentsMaxTime\
${csv_delim}PipelineRunAvgTime\
${csv_delim}PipelineRunMaxTime\
${csv_delim}IntegrationTestsRunPipelineSucceededTimeAvg\
${csv_delim}IntegrationTestsRunPipelineSucceededTimeMax\
${csv_delim}DeploymentSucceededTimeAvg\
${csv_delim}DeploymentSucceededTimeMax\
${csv_delim}ClusterCPUUsageAvg\
${csv_delim}ClusterDiskUsageAvg\
${csv_delim}ClusterMemoryUsageAvg\
${csv_delim}ClusterPodCountAvg\
${csv_delim}ClusterNodesWorkerCountAvg\
${csv_delim}ClusterRunningPodsOnWorkersCountAvg\
${csv_delim}ClusterPVCInUseAvg\
${csv_delim}TektonResultsWatcherMemoryMin\
${csv_delim}TektonResultsWatcherMemoryMax\
${csv_delim}TektonResultsWatcherMemoryRange\
${tekton_results_watcher_pod_headers}\
${csv_delim}SchedulerPendingPodsCountAvg\
${csv_delim}TokenPoolRatePrimaryAvg\
${csv_delim}TokenPoolRateSecondaryAvg\
${csv_delim}ClusterPipelineRunCountAvg\
${csv_delim}ClusterPipelineWorkqueueDepthAvg\
${csv_delim}ClusterPipelineScheduleFirstPodAvg\
${csv_delim}ClusterTaskRunThrottledByNodeResourcesAvg\
${csv_delim}ClusterTaskRunThrottledByDefinedQuotaAvg\
${csv_delim}EtcdRequestDurationSecondsAvg\
${csv_delim}ClusterNetworkBytesTotalAvg\
${csv_delim}ClusterNetworkReceiveBytesTotalAvg\
${csv_delim}ClusterNetworkTransmitBytesTotalAvg\
${csv_delim}NodeDiskIoTimeSecondsTotalAvg" \
        >"$max_concurrency_csv"
    mc_files=$(find "$output_dir" -type f -name 'load-tests.max-concurrency.*.json')
    if [ -n "$mc_files" ]; then
        for i in $mc_files; do
            iteration_index=$(echo "$i" | sed -e 's,'"$output_dir"'/load-tests.max-concurrency.\([0-9-]\+\).*,\1,g')

            parked_go_routines=$(get_parked_go_routines "$iteration_index")
            parked_go_routines_columns=""
            if [ -n "$parked_go_routines" ]; then
                for g in $parked_go_routines; do
                    parked_go_routines_columns="$parked_go_routines_columns + $csv_delim_quoted + \"$g\""
                done
            else
                for _ in $(seq 1 "$(get_tekton_results_watcher_pod_count)"); do
                    parked_go_routines_columns="$parked_go_routines_columns + $csv_delim_quoted"
                done
            fi
            jq -rc "(.metadata.\"max-concurrency\".iteration | tostring) \
                + $csv_delim_quoted + (.threads | tostring) \
                + $csv_delim_quoted + (.workloadKPI | tostring) \
                + $csv_delim_quoted + (.errorsTotal | tostring) \
                + $csv_delim_quoted + (.createUserTimeAvg | tostring) \
                + $csv_delim_quoted + (.createUserTimeMax | tostring) \
                + $csv_delim_quoted + (.createApplicationsTimeAvg | tostring) \
                + $csv_delim_quoted + (.createApplicationsTimeMax | tostring) \
                + $csv_delim_quoted + (.createCDQsTimeAvg | tostring) \
                + $csv_delim_quoted + (.createCDQsTimeMax | tostring) \
                + $csv_delim_quoted + (.createComponentsTimeAvg | tostring) \
                + $csv_delim_quoted + (.createComponentsTimeMax | tostring) \
                + $csv_delim_quoted + (.runPipelineSucceededTimeAvg | tostring) \
                + $csv_delim_quoted + (.runPipelineSucceededTimeMax | tostring) \
                + $csv_delim_quoted + (.integrationTestsRunPipelineSucceededTimeAvg | tostring) \
                + $csv_delim_quoted + (.integrationTestsRunPipelineSucceededTimeMax | tostring) \
                + $csv_delim_quoted + (.deploymentSucceededTimeAvg | tostring) \
                + $csv_delim_quoted + (.deploymentSucceededTimeMax | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_cpu_usage_seconds_total_rate.mean | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_disk_throughput_total.mean | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_memory_usage_rss_total.mean | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_pods_count.mean | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_nodes_worker_count.mean | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_running_pods_on_workers_count.mean | tostring) \
                + $csv_delim_quoted + (.measurements.storage_count_attachable_volumes_in_use.mean | tostring) \
                + $csv_delim_quoted + (.measurements.\"tekton-results-watcher\".\"container[watcher]\".memory.min | tostring) \
                + $csv_delim_quoted + (.measurements.\"tekton-results-watcher\".\"container[watcher]\".memory.max | tostring) \
                + $csv_delim_quoted + (.measurements.\"tekton-results-watcher\".\"container[watcher]\".memory.range | tostring) \
                ${parked_go_routines_columns} \
                + $csv_delim_quoted + (.measurements.scheduler_pending_pods_count.mean | tostring) \
                + $csv_delim_quoted + (.measurements.token_pool_rate_primary.mean | tostring) \
                + $csv_delim_quoted + (.measurements.token_pool_rate_secondary.mean | tostring) \
                + $csv_delim_quoted + (.measurements.tekton_pipelines_controller_running_pipelineruns_count.mean | tostring) \
                + $csv_delim_quoted + (.measurements.tekton_tekton_pipelines_controller_workqueue_depth.mean | tostring) \
                + $csv_delim_quoted + (.measurements.pipelinerun_duration_scheduled_seconds.mean | tostring) \
                + $csv_delim_quoted + (.measurements.tekton_pipelines_controller_running_taskruns_throttled_by_node.mean | tostring) \
                + $csv_delim_quoted + (.measurements.tekton_pipelines_controller_running_taskruns_throttled_by_quota.mean | tostring) \
                + $csv_delim_quoted + (.measurements.etcd_request_duration_seconds_average.mean | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_network_bytes_total.mean | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_network_receive_bytes_total.mean | tostring) \
                + $csv_delim_quoted + (.measurements.cluster_network_transmit_bytes_total.mean | tostring) \
                + $csv_delim_quoted + (.measurements.node_disk_io_time_seconds_total.mean | tostring)" \
                "$i" >>"$max_concurrency_csv"
        done
    else
        echo "WARNING: No file matching '$output_dir/load-tests.max-concurrency.*.json' found!"
    fi
}

get_parked_go_routines() {
    goroutines_pprof=$(find "$output_dir" -name "tekton-results-watcher.tekton-results-watcher-*.goroutine-dump-0.$1.pprof")
    count=0
    for i in $goroutines_pprof; do
        if [ $count -gt 0 ]; then
            echo -n " "
        fi
        echo -n "$(go tool pprof -text "$i" 2>/dev/null | grep 'runtime.gopark$' | sed -e 's,[ ]*\([0-9]\+\) .*,\1,g')"
        count=$((count + 1))
    done
}

collect_timestamp_csvs() {
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
    oc get pipelineruns.tekton.dev -A -o json | jq "$jq_cmd" | sed -e "s/\n//g" -e "s/^\"//g" -e "s/\"$//g" -e "s/Z;/;/g" | sort -t ";" -k 13 -r -n >>"$pipelinerun_timestamps"
}

echo "Collecting max concurrency results..."
collect_artifacts || true
collect_timestamp_csvs || true
collect_monitoring_data || true
collect_scalability_data || true
collect_tekton_profiling_data || true
popd

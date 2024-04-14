#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# Just a helper script to output CSV file based on all found benchmark JSON files
headers="BUILD_ID,\
Started,\
Ended,\
\
users_per_thread,\
threads,\
total_users,\
component_repo,\
build_pipeline_selector_bundle,\
user_prefix,\
scenario,\
job_name,\
openshift_api,\
\
control_plane_count,\
control_plane_flavor,\
compute_plane_count,\
compute_plane_flavor,\
\
createUserSuccesses,\
createUserFailures,\
createUserFailureRate,\
createUserTimeAvg,\
createUserTimeMax,\
\
createApplicationsSuccesses,\
createApplicationsFailures,\
createApplicationsFailureRate,\
createApplicationsTimeAvg,\
createApplicationsTimeMax,\
\
createCDQsSuccesses,\
createCDQsFailures,\
createCDQsFailureRate,\
createCDQsTimeAvg,\
createCDQsTimeMax,\
\
createComponentsSuccesses,\
createComponentsFailures,\
createComponentsFailureRate,\
createComponentsTimeAvg,\
createComponentsTimeMax,\
\
runPipelineSuccesses,\
runPipelineFailures,\
runPipelineFailureRate,\
runPipelineSucceededTimeAvg,\
runPipelineSucceededTimeMax,\
\
integrationTestsRunPipelineSuccesses,\
integrationTestsRunPipelineFailures,\
integrationTestsRunPipelineFailureRate,\
integrationTestsRunPipelineSucceededTimeAvg,\
integrationTestsRunPipelineSucceededTimeMax,\
\
deploymentSuccesses,\
deploymentFailures,\
deploymentFailureRate,\
deploymentSucceededTimeAvg,\
deploymentSucceededTimeMax,\
\
TOTAL_AVG,\
WORKLOAD_KPI,\
\
.measurements.cluster_cpu_usage_seconds_total_rate.mean,\
.measurements.cluster_disk_throughput_total.mean,\
.measurements.cluster_memory_usage_rss_total.mean,\
.measurements.cluster_network_bytes_total.mean,\
.measurements.cluster_network_receive_bytes_total.mean,\
.measurements.cluster_network_transmit_bytes_total.mean,\
.measurements.cluster_nodes_worker_count.mean,\
.measurements.cluster_pods_count.mean,\
.measurements.cluster_running_pods_on_workers_count.mean,\
.measurements.etcd_request_duration_seconds_average.mean,\
.measurements.node_disk_io_time_seconds_total.mean,\
.measurements.pipelinerun_duration_scheduled_seconds.mean,\
.measurements.scheduler_pending_pods_count.mean,\
.measurements.storage_count_attachable_volumes_in_use.mean,\
.measurements.tekton_pipelines_controller_running_pipelineruns_count.mean,\
.measurements.tekton_pipelines_controller_running_taskruns_throttled_by_node.mean,\
.measurements.tekton_pipelines_controller_running_taskruns_throttled_by_quota.mean,\
.measurements.tekton_tekton_pipelines_controller_workqueue_depth.mean,\
\
.measurements.tekton-pipelines-controller.count_ready.mean,\
.measurements.tekton-pipelines-controller.restarts.mean,\
.measurements.tekton-pipelines-controller.cpu.mean,\
.measurements.tekton-pipelines-controller.memory.mean,\
.measurements.tekton-pipelines-controller.disk_throughput.mean,\
.measurements.tekton-pipelines-controller.network_throughput.mean\
"
echo "$headers"

find "${1:-.}" -name load-tests.json -print0 | while IFS= read -r -d '' filename; do
    grep --quiet "XXXXX" "${filename}" && echo "WARNING placeholders found in ${filename}, removing"
    sed -Ee 's/: ([0-9]+\.[0-9]*[X]+[0-9e\+-]*|[0-9]*X+[0-9]*\.[0-9e\+-]*|[0-9]*X+[0-9]*\.[0-9]*X+[0-9e\+-]+)/: "\1"/g' "${filename}" \
        | jq --raw-output '[
        .metadata.env.BUILD_ID,
        .timestamp,
        .endTimestamp,

        .metadata.scenario.USERS_PER_THREAD,
        .metadata.scenario.THREADS,
        (.metadata.scenario.USERS_PER_THREAD|tonumber) * (.metadata.scenario.THREADS|tonumber),
        .metadata.scenario.COMPONENT_REPO,
        .metadata.scenario.BUILD_PIPELINE_SELECTOR_BUNDLE,
        .metadata.scenario.USER_PREFIX,
        .metadata.env.SCENARIO,
        .metadata.env.JOB_NAME,
        .metadata.env.OPENSHIFT_API,

        .metadata.cluster."control-plane".count,
        .metadata.cluster."control-plane".flavor,
        .metadata.cluster."compute-nodes".count,
        .metadata.cluster."compute-nodes".flavor,

        .createUserSuccesses,
        .createUserFailures,
        .createUserFailureRate,
        .createUserTimeAvg,
        .createUserTimeMax,

        .createApplicationsSuccesses,
        .createApplicationsFailures,
        .createApplicationsFailureRate,
        .createApplicationsTimeAvg,
        .createApplicationsTimeMax,

        .createCDQsSuccesses,
        .createCDQsFailures,
        .createCDQsFailureRate,
        .createCDQsTimeAvg,
        .createCDQsTimeMax,

        .createComponentsSuccesses,
        .createComponentsFailures,
        .createComponentsFailureRate,
        .createComponentsTimeAvg,
        .createComponentsTimeMax,

        .runPipelineSuccesses,
        .runPipelineFailures,
        .runPipelineFailureRate,
        .runPipelineSucceededTimeAvg,
        .runPipelineSucceededTimeMax,

        .integrationTestsRunPipelineSuccesses,
        .integrationTestsRunPipelineFailures,
        .integrationTestsRunPipelineFailureRate,
        .integrationTestsRunPipelineSucceededTimeAvg,
        .integrationTestsRunPipelineSucceededTimeMax,

        .deploymentSuccesses,
        .deploymentFailures,
        .deploymentFailureRate,
        .deploymentSucceededTimeAvg,
        .deploymentSucceededTimeMax,

        .createApplicationsTimeAvg + .createCDQsTimeAvg + .createComponentsTimeAvg + .integrationTestsRunPipelineSucceededTimeAvg + .runPipelineSucceededTimeAvg + .deploymentSucceededTimeAvg,
        .workloadKPI,

        .measurements.cluster_cpu_usage_seconds_total_rate.mean,
        .measurements.cluster_disk_throughput_total.mean,
        .measurements.cluster_memory_usage_rss_total.mean,
        .measurements.cluster_network_bytes_total.mean,
        .measurements.cluster_network_receive_bytes_total.mean,
        .measurements.cluster_network_transmit_bytes_total.mean,
        .measurements.cluster_nodes_worker_count.mean,
        .measurements.cluster_pods_count.mean,
        .measurements.cluster_running_pods_on_workers_count.mean,
        .measurements.etcd_request_duration_seconds_average.mean,
        .measurements.node_disk_io_time_seconds_total.mean,
        .measurements.pipelinerun_duration_scheduled_seconds.mean,
        .measurements.scheduler_pending_pods_count.mean,
        .measurements.storage_count_attachable_volumes_in_use.mean,
        .measurements.tekton_pipelines_controller_running_pipelineruns_count.mean,
        .measurements.tekton_pipelines_controller_running_taskruns_throttled_by_node.mean,
        .measurements.tekton_pipelines_controller_running_taskruns_throttled_by_quota.mean,
        .measurements.tekton_tekton_pipelines_controller_workqueue_depth.mean,

        .measurements."tekton-pipelines-controller".count_ready.mean,
        .measurements."tekton-pipelines-controller".restarts.mean,
        .measurements."tekton-pipelines-controller".cpu.mean,
        .measurements."tekton-pipelines-controller".memory.mean,
        .measurements."tekton-pipelines-controller".disk_throughput.mean,
        .measurements."tekton-pipelines-controller".network_throughput.mean
        ] | @csv' &&
        rc=0 || rc=1
    if [[ "$rc" -ne 0 ]]; then
        echo "ERROR failed on ${filename}"
    fi
done

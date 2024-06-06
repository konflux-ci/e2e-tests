#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# shellcheck disable=SC1090
source "/usr/local/ci-secrets/redhat-appstudio-load-test/load-test-scenario.${1:-concurrent}"

pushd "${2:-.}"

output_dir="${OUTPUT_DIR:-./tests/load-tests}"

source "$( dirname $0 )/utils.sh"
source "$( dirname $0 )/user-prefix.sh"

echo "Collecting load test results"
load_test_log=$ARTIFACT_DIR/load-tests.log
find "$output_dir" -type f -name '*.log' -exec cp -vf {} "${ARTIFACT_DIR}" \;
find "$output_dir" -type f -name 'load-tests.json' -exec cp -vf {} "${ARTIFACT_DIR}" \;
find "$output_dir" -type f -name 'gh-rate-limits-remaining.csv' -exec cp -vf {} "${ARTIFACT_DIR}" \;
find "$output_dir" -type f -name '*.pprof' -exec cp -vf {} "${ARTIFACT_DIR}" \;

application_stub=$ARTIFACT_DIR/collected-applications.appstudio.redhat.com
component_stub=$ARTIFACT_DIR/collected-components.appstudio.redhat.com
pipelinerun_stub=$ARTIFACT_DIR/collected-pipelineruns.tekton.dev
taskrun_stub=$ARTIFACT_DIR/collected-taskruns.tekton.dev
pod_stub=$ARTIFACT_DIR/collected-pods
node_stub=$ARTIFACT_DIR/collected-nodes

application_service_log=$ARTIFACT_DIR/application-service.log
application_service_log_segments=$ARTIFACT_DIR/application-service-log-segments
monitoring_collection_log=$ARTIFACT_DIR/monitoring-collection.log
monitoring_collection_data=$ARTIFACT_DIR/load-tests.json
monitoring_collection_dir=$ARTIFACT_DIR/monitoring-collection-raw-data-dir
mkdir -p "$monitoring_collection_dir"
csv_delim=";"
csv_delim_quoted="\"$csv_delim\""
dt_format='"%Y-%m-%dT%H:%M:%SZ"'

## Application info
echo "Collecting Application timestamps..."
collect_application "-A" "$application_stub"

## Component info
echo "Collecting Component timestamps..."
collect_component "-A" "$component_stub"

## PipelineRun info
echo "Collecting PipelineRun timestamps..."
collect_pipelinerun "-A" "$pipelinerun_stub"

## TaskRun info
echo "Collecting TaskRun timestamps..."
collect_taskrun "-A" "$taskrun_stub"

## Pods info
echo "Collecting pod distribution over nodes"
collect_pods "-A" "$pod_stub"

## Nodes info
echo "Collecting node specs"
collect_nodes "$node_stub"

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
    echo "WARNING: File $load_test_log not found!"
fi

## Monitoring data
echo "Setting up tool to collect monitoring data..."
python3 -m venv venv
set +u
source venv/bin/activate
set -u
python3 -m pip install -U pip
python3 -m pip install -e "git+https://github.com/redhat-performance/opl.git#egg=opl-rhcloud-perf-team-core&subdirectory=core"

echo "Collecting monitoring data..."
if [ -f "$monitoring_collection_data" ]; then
    mstart=$(date --utc --date "$(status_data.py --status-data-file "$monitoring_collection_data" --get timestamp)" --iso-8601=seconds)
    mend=$(date --utc --date "$(status_data.py --status-data-file "$monitoring_collection_data" --get endTimestamp)" --iso-8601=seconds)
    mhost=$(oc -n openshift-monitoring get route -l app.kubernetes.io/name=thanos-query -o json | jq --raw-output '.items[0].spec.host')
    status_data.py \
        --status-data-file "$monitoring_collection_data" \
        --additional ./tests/load-tests/cluster_read_config.yaml \
        --monitoring-start "$mstart" \
        --monitoring-end "$mend" \
        --monitoring-raw-data-dir "$monitoring_collection_dir" \
        --prometheus-host "https://$mhost" \
        --prometheus-port 443 \
        --prometheus-token "$(oc whoami -t)" \
        -d &>$monitoring_collection_log
    set +u
    deactivate
    set -u
else
    echo "WARNING: File $monitoring_collection_data not found!"
fi

if [ "${TEKTON_PERF_ENABLE_CPU_PROFILING:-}" == "true" ] || [ "${TEKTON_PERF_ENABLE_MEMORY_PROFILING:-}" == "true" ]; then
    echo "Collecting profiling data from Tekton"
    for pprof_profile in $(find "$output_dir" -name "*.pprof"); do
        file=$(basename "$pprof_profile")
        go tool pprof -text "$pprof_profile" >"$ARTIFACT_DIR/$file.txt" || true
        go tool pprof -svg -output="$ARTIFACT_DIR/$file.svg" "$pprof_profile" || true
    done
fi

## Tekton Artifact Performance Analysis
tapa_dir=./tapa.git

echo "Installing Tekton Artifact Performance Analysis (tapa)"
rm -rf "$tapa_dir"
git clone https://github.com/gabemontero/tekton-artifact-performance-analysis "$tapa_dir"
pushd "$tapa_dir"
go mod tidy
go mod vendor
go build -o tapa . && chmod +x ./tapa
popd
export PATH="$PATH:$tapa_dir"

tapa="tapa -t csv"

echo "Running Tekton Artifact Performance Analysis"
tapa_prlist_csv=$ARTIFACT_DIR/tapa.prlist.csv
tapa_trlist_csv=$ARTIFACT_DIR/tapa.trlist.csv
tapa_podlist_csv=$ARTIFACT_DIR/tapa.podlist.csv
tapa_podlist_containers_csv=$ARTIFACT_DIR/tapa.podlist.containers.csv
tapa_all_csv=$ARTIFACT_DIR/tapa.all.csv
tapa_tmp=tapa.tmp

sort_csv() {
    if [ -f "$1" ]; then
        head -n1 "$1" >"$2"
        tail -n+2 "$1" | sort -t ";" -k 2 -r -n >>"$2"
    else
        echo "WARNING: File $1 not found!"
    fi
}

$tapa prlist "${pipelinerun_stub}.json" >"$tapa_tmp"
sort_csv "$tapa_tmp" "$tapa_prlist_csv"

$tapa trlist "${taskrun_stub}.json" >"$tapa_tmp"
sort_csv "$tapa_tmp" "$tapa_trlist_csv"

$tapa podlist "${pod_stub}.json" >"$tapa_tmp"
sort_csv "$tapa_tmp" "$tapa_podlist_csv"

$tapa podlist --containers-only "${pod_stub}.json" >"$tapa_tmp"
sort_csv "$tapa_tmp" "$tapa_podlist_containers_csv"

$tapa all "${pipelinerun_stub}.json" "${taskrun_stub}.json" "${pod_stub}.json" >"$tapa_tmp"
sort_csv "$tapa_tmp" "$tapa_all_csv"

popd

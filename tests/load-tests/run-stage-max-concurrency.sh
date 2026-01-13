#!/bin/bash
export MY_GITHUB_ORG GITHUB_TOKEN

OUTPUT_DIR="${OUTPUT_DIR:-.}"

export TEKTON_PERF_PROFILE_CPU_PERIOD=${TEKTON_PERF_PROFILE_CPU_PERIOD:-${THRESHOLD:-300}}

export THRESHOLD=${THRESHOLD:-3500}
export THRESHOLD_ERR=${THRESHOLD_ERR:-30}

load_test() {
    local workdir threads index
    workdir=${1:-/tmp}
    threads=${2:-1}
    index=$(printf "%04d" "$threads")
    ## Enable CPU profiling in Tekton
    if [ "${TEKTON_PERF_ENABLE_CPU_PROFILING:-}" == "true" ]; then
        echo "Starting CPU profiling with pprof"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            file="tekton-pipelines-controller.$pod.cpu-profile"
            oc exec -n openshift-pipelines "$p" -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/profile?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$workdir/$file.pprof" &
            echo $! >"$workdir/$file.pid"
        done
        for p in $(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name); do
            pod="${p##*/}"
            file="tekton-results-watcher.$pod.cpu-profile"
            oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/profile?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$workdir/$file.pprof" &
            echo $! >"$workdir/$file.pid"
        done
    fi
    ## Enable memory profiling in Tekton
    if [ "${TEKTON_PERF_ENABLE_MEMORY_PROFILING:-}" == "true" ]; then
        echo "Starting memory profiling of Tekton controller with pprof"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            file="tekton-pipelines-controller.$pod.memory-profile"
            oc exec -n openshift-pipelines "$p" -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/heap?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$workdir/$file.pprof" &
            echo $! >"$workdir/$file.pid"
        done
        echo "Starting memory profiling of Tekton results watcher with pprof"
        for p in $(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name); do
            pod="${p##*/}"
            file="tekton-results-watcher.$pod.memory-profile"
            oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/heap?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$workdir/$file.pprof" &
            echo $! >"$workdir/$file.pid"
        done
    fi
    rm -rvf "$workdir/load-test.json"
    rm -rvf "$workdir/load-test.log"

    options=""
    [[ -n "${PIPELINE_IMAGE_PULL_SECRETS:-}" ]] && for s in $PIPELINE_IMAGE_PULL_SECRETS; do options="$options --pipeline-image-pull-secrets $s"; done

    date -Ins --utc >started
    go run loadtest.go \
        --applications-count "${APPLICATIONS_COUNT:-1}" \
        --build-pipeline-selector-bundle "${BUILD_PIPELINE_SELECTOR_BUNDLE:-}" \
        --component-repo "${COMPONENT_REPO:-https://github.com/rhtap-perf-test/nodejs-devfile-sample}" \
        --concurrency "$threads" \
        --journey-duration "${JOURNEY_DURATION:-1h}" \
        --journey-repeats "${JOURNEY_REPEATS:-1}" \
        --log-"${LOGGING_LEVEL:-info}" \
        --stage \
        --pipeline-repo-templating="${PIPELINE_REPO_TEMPLATING:-false}" \
        --output-dir "${workdir:-/tmp}" \
        --purge="${PURGE:-true}" \
        --quay-repo "${QUAY_REPO:-stonesoup_perfscale}" \
        --test-scenario-git-url "${TEST_SCENARIO_GIT_URL:-https://github.com/konflux-ci/integration-examples.git}" \
        --test-scenario-path-in-repo "${TEST_SCENARIO_PATH_IN_REPO:-pipelines/integration_resolver_pipeline_pass.yaml}" \
        --test-scenario-revision "${TEST_SCENARIO_REVISION:-main}" \
        --waitintegrationtestspipelines="${WAIT_INTEGRATION_TESTS:-true}" \
        --waitpipelines="${WAIT_PIPELINES:-true}" \
        $options \
        2>&1 | tee "$workdir/load-test.log"
    
    # Capture and exit if there are unexpected errors in loadtest.go
    LOADTEST_EXIT_STATUS=${PIPESTATUS[0]}
    if [ ${LOADTEST_EXIT_STATUS} -ne 0 ]; then
        echo "[$(date --utc -Ins)] loadtest.go exited with non-zero (${LOADTEST_EXIT_STATUS}) status code."
        exit 1
    fi

    date -Ins --utc >ended

    set +u
    source venv/bin/activate
    set -u

    echo "[$(date --utc -Ins)] Create summary JSON with timings"
    ./evaluate.py "$workdir/load-test-timings.csv" "$workdir/load-test-timings.json"

    echo "[$(date --utc -Ins)] Creating main status data file"
    STATUS_DATA_FILE="$workdir/load-test.json"
    status_data.py \
        --status-data-file "${STATUS_DATA_FILE}" \
        --set "name=Konflux loadtest" "started=$(cat started)" "ended=$(cat ended)" \
        --set-subtree-json "parameters.options=$workdir/load-test-options.json" "results.measurements=$workdir/load-test-timings.json"

    deactivate

    if [ "${TEKTON_PERF_ENABLE_CPU_PROFILING:-}" == "true" ] || [ "${TEKTON_PERF_ENABLE_MEMORY_PROFILING:-}" == "true" ]; then
        echo "[$(date --utc -Ins)] Waiting for the Tekton profiling to finish up to ${TEKTON_PERF_PROFILE_CPU_PERIOD}s"
        for pid_file in $(find "$workdir" -name 'tekton*.pid'); do
            wait "$(cat "$pid_file")"
            rm -rvf "$pid_file"
        done
        echo "[$(date --utc -Ins)] Getting Tekton controller goroutine dump"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            for i in 0 1 2; do
                file="tekton-pipelines-controller.$pod.goroutine-dump-$i"
                oc exec -n tekton-results "$p" -- bash -c "curl -SsL localhost:8008/debug/pprof/goroutine?debug=$i | base64" | base64 -d >"$workdir/$file.pprof"
            done
        done
        echo "[$(date --utc -Ins)] Getting Tekton results watcher goroutine dump"
        for p in $(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name); do
            pod="${p##*/}"
            for i in 0 1 2; do
                file="tekton-results-watcher.$pod.goroutine-dump-$i"
                oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL localhost:8008/debug/pprof/goroutine?debug=$i | base64" | base64 -d >"$workdir/$file.pprof"
            done
        done
    fi

    echo "[$(date --utc -Ins)] Finished processing results"
}

max_concurrency() {
    local iteration index iteration_index
    output="$OUTPUT_DIR/load-test.max-concurrency.json"
    IFS="," read -r -a maxConcurrencySteps <<<"$(echo "${MAX_CONCURRENCY_STEPS:-1\ 5\ 10\ 25\ 50\ 100\ 150\ 200}" | sed 's/ /,/g')"
    maxThreads=${MAX_THREADS:-10}      # Do not go above this concurrency.
    threshold_sec=${THRESHOLD:-300}    # In seconds. If KPI crosses this duration, stop.
    threshold_err=${THRESHOLD_ERR:-10} # Failure ratio. When crossed, stop.
    echo "[$(date --utc -Ins)] Starting max-concurrency test on Stage"
    echo '{"started":"'"$(date +%FT%T%:z)"'", "maxThreads": '"$maxThreads"', "maxConcurrencySteps": "'"${maxConcurrencySteps[*]}"'", "threshold": '"$threshold_sec"', "thresholdErrors": '"$threshold_err"', "maxConcurrencyReached": 0, "computedConcurrency": 0, "workloadKPI": 0, "ended": "", "errorsTotal": -1}' | jq >"$output"
    iteration=0

    {
        python3 -m venv venv
        set +u
        source venv/bin/activate
        set -u
        python3 -m pip install -U pip
        python3 -m pip install -e "git+https://github.com/redhat-performance/opl.git#egg=opl-rhcloud-perf-team-core&subdirectory=core"
        python3 -m pip install tabulate
        python3 -m pip install matplotlib
        deactivate
    } &>"$OUTPUT_DIR/monitoring-setup.log"

    for t in "${maxConcurrencySteps[@]}"; do
        iteration="$((iteration + 1))"
        if (("$t" > "$maxThreads")); then
            break
        fi
        echo "[$(date --utc -Ins)] Starting iteration ${iteration} with concurrency ${t}"
        iteration_index="$(printf "%04d" "$iteration")-$(printf "%04d" "$t")"
        workdir="${OUTPUT_DIR}/iteration-${iteration_index}"
        mkdir "${workdir}"
        load_test "$workdir" "$t"
        jq ".metadata.\"max-concurrency\".iteration = \"$(printf "%04d" "$iteration")\"" "$workdir/load-test.json" >"$OUTPUT_DIR/$$.json" && mv -f "$OUTPUT_DIR/$$.json" "$workdir/load-test.json"
        workloadKPI=$(jq '.results.measurements.KPI.mean' "$workdir/load-test.json")
        workloadKPIerrors=$(jq '.results.measurements.KPI.errors' "$workdir/load-test.json")
        if [ -z "$workloadKPI" ] || [ -z "$workloadKPIerrors" ] || [ "$workloadKPI" = "null" ] || [ "$workloadKPIerrors" = "null" ]; then
            echo "[$(date --utc -Ins)] Error getting test iteration results: $workloadKPI/$workloadKPIerrors"
            exit 1
        elif awk "BEGIN { exit !($workloadKPI == -1 || $workloadKPI > $threshold_sec || $workloadKPIerrors / $t * 100 > $threshold_err)}"; then
            echo "[$(date --utc -Ins)] The average time a workload took to succeed (${workloadKPI}s) or error rate (${workloadKPIerrors}/${t}) has exceeded a threshold of ${threshold_sec}s or ${threshold_err} error rate with $t threads."
            workloadKPIOld=$(jq '.workloadKPI' "$output")
            threadsOld=$(jq '.maxConcurrencyReached' "$output")
            computedConcurrency=$(python3 -c "import sys; t = float(sys.argv[1]); a = float(sys.argv[2]); b = float(sys.argv[3]); c = float(sys.argv[4]); d = float(sys.argv[5]); print((t - b) / ((d - b) / (c - a)) + a)" "$threshold_sec" "$threadsOld" "$workloadKPIOld" "$t" "$workloadKPI")
            jq ".computedConcurrency = $computedConcurrency" "$output" >"$OUTPUT_DIR/$$.json" && mv -f "$OUTPUT_DIR/$$.json" "$output"
            break
        else
            echo "[$(date --utc -Ins)] The average time a workload took to succeed (${workloadKPI}s) and error rate (${workloadKPIerrors}/${t}) looks good with $t threads."
            jq ".maxConcurrencyReached = $t" "$output" >"$OUTPUT_DIR/$$.json" && mv -f "$OUTPUT_DIR/$$.json" "$output"
            jq ".workloadKPI = $workloadKPI" "$output" >"$OUTPUT_DIR/$$.json" && mv -f "$OUTPUT_DIR/$$.json" "$output"
            jq ".computedConcurrency = $t" "$output" >"$OUTPUT_DIR/$$.json" && mv -f "$OUTPUT_DIR/$$.json" "$output"
            jq '.ended = "'"$(date +%FT%T%:z)"'"' "$output" >"$OUTPUT_DIR/$$.json" && mv -f "$OUTPUT_DIR/$$.json" "$output"
            jq ".errorsTotal = $workloadKPIerrors" "$output" >"$OUTPUT_DIR/$$.json" && mv -f "$OUTPUT_DIR/$$.json" "$output"
        fi
    done
}

max_concurrency

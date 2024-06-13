#!/bin/bash
export MY_GITHUB_ORG GITHUB_TOKEN

output_dir="${OUTPUT_DIR:-.}"

export TEKTON_PERF_PROFILE_CPU_PERIOD=${TEKTON_PERF_PROFILE_CPU_PERIOD:-${THRESHOLD:-300}}

OPENSHIFT_API="${OPENSHIFT_API:-$(yq '.clusters[0].cluster.server' "$KUBECONFIG")}"
OPENSHIFT_USERNAME="${OPENSHIFT_USERNAME:-kubeadmin}"
OPENSHIFT_PASSWORD="${OPENSHIFT_PASSWORD:-$(cat "$KUBEADMIN_PASSWORD_FILE")}"

load_test() {
    local threads iteration index iteration_index
    threads=${1:-1}
    iteration=$(printf "%04d" "${2:-1}")
    index=$(printf "%04d" "$threads")
    iteration_index="${iteration}-${index}"
    ## Enable CPU profiling in Tekton
    if [ "${TEKTON_PERF_ENABLE_CPU_PROFILING:-}" == "true" ]; then
        echo "Starting CPU profiling with pprof"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            file="tekton-pipelines-controller.$pod.cpu-profile.$iteration_index"
            oc exec -n openshift-pipelines "$p" -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/profile?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/$file.pprof" &
            echo $! >"$output_dir/$file.pid"
        done
        for p in $(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name); do
            pod="${p##*/}"
            file=tekton-results-watcher.$pod.cpu-profile.$iteration_index
            oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/profile?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/$file.pprof" &
            echo $! >"$output_dir/$file.pid"
        done
    fi
    ## Enable memory profiling in Tekton
    if [ "${TEKTON_PERF_ENABLE_MEMORY_PROFILING:-}" == "true" ]; then
        echo "Starting memory profiling of Tekton controller with pprof"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            file="tekton-pipelines-controller.$pod.memory-profile.$iteration_index"
            oc exec -n openshift-pipelines "$p" -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/heap?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/$file.pprof" &
            echo $! >"$output_dir/$file.pid"
        done
        echo "Starting memory profiling of Tekton results watcher with pprof"
        for p in $(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name); do
            pod="${p##*/}"
            file=tekton-results-watcher.$pod.memory-profile.$iteration_index
            oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/heap?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/$file.pprof" &
            echo $! >"$output_dir/$file.pid"
        done
    fi
    rm -rvf "$output_dir/load-test.json"
    rm -rvf "$output_dir/load-test.log"
    go run loadtest.go \
        --component-repo "${COMPONENT_REPO:-https://github.com/nodeshift-starters/devfile-sample.git}" \
        --username "$USER_PREFIX-$index" \
        --users 1 \
        -w="${WAIT_PIPELINES:-true}" \
        -i="${WAIT_INTEGRATION_TESTS:-false}" \
        -d="${WAIT_DEPLOYMENTS:-false}" \
        -l \
        -o "$output_dir" \
        -t "$threads" \
        --disable-metrics="${DISABLE_METRICS:-false}" \
        --pushgateway-url "${PUSHGATEWAY_URL:-rhtapqe.com}" \
        --enable-progress-bars="${ENABLE_PROGRESS_BARS:-false}" \
        --pipeline-skip-initial-checks="${PIPELINE_SKIP_INITIAL_CHECKS:-true}"
    if [ "${TEKTON_PERF_ENABLE_CPU_PROFILING:-}" == "true" ] || [ "${TEKTON_PERF_ENABLE_MEMORY_PROFILING:-}" == "true" ]; then
        echo "Waiting for the Tekton profiling to finish up to ${TEKTON_PERF_PROFILE_CPU_PERIOD}s"
        for pid_file in $(find "$output_dir" -name 'tekton*.pid'); do
            wait "$(cat "$pid_file")"
            rm -rvf "$pid_file"
        done
        echo "Getting Tekton controller goroutine dump"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            for i in 0 1 2; do
                file="tekton-pipelines-controller.$pod.goroutine-dump-$i.$iteration_index"
                oc exec -n tekton-results "$p" -- bash -c "curl -SsL localhost:8008/debug/pprof/goroutine?debug=$i | base64" | base64 -d >"$output_dir/$file.pprof"
            done
        done
        echo "Getting Tekton results watcher goroutine dump"
        for p in $(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name); do
            pod="${p##*/}"
            for i in 0 1 2; do
                file="tekton-results-watcher.$pod.goroutine-dump-$i.$iteration_index"
                oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL localhost:8008/debug/pprof/goroutine?debug=$i | base64" | base64 -d >"$output_dir/$file.pprof"
            done
        done
    fi
}

remove_finalizers() {
    res=$1
    while [ "$(oc get "$res" -A -o json | jq -rc '.items[] | select(.metadata.namespace | startswith("'"$USER_PREFIX"'"))' | wc -l)" != "0" ]; do
        echo "## Removing finalizers for all $res"
        while read -r line; do
            echo "$line '{\"metadata\":{\"finalizers\":[]}}' --type=merge;"
        done <<<"$(oc get "$res" -A -o json | jq -rc '.items[] | select(.metadata.namespace | startswith("'"$USER_PREFIX"'")) | "oc patch '"$res"' " + .metadata.name + " -n " + .metadata.namespace + " -p "')" | bash -s
    done
}

clean_namespaces() {
    echo "Deleting resources from previous steps"
    for res in pipelineruns.tekton.dev components.appstudio.redhat.com componentdetectionqueries.appstudio.redhat.com snapshotenvironmentbindings.appstudio.redhat.com applications.appstudio.redhat.com; do
        echo -e " * $res"
        if [ -n "${DELETE_INCLUDE_FINALIZERS:-}" ]; then
            remove_finalizers "$res" &
            echo "## Deleting all $res"
        fi
        oc get "$res" -A -o json | jq -rc '.items[] | select(.metadata.namespace | startswith("'"$USER_PREFIX"'"))| "oc delete '"$res"' " + .metadata.name + " -n " + .metadata.namespace + " --ignore-not-found=true;"' | bash -s
    done
    oc get usersignups.toolchain.dev.openshift.com -A -o name | grep "$USER_PREFIX" | xargs oc delete -n toolchain-host-operator --ignore-not-found=true
    attempts=60
    attempt=1
    sleep="5s"
    while [ "$attempt" -le "$attempts" ]; do
        echo " * Waiting $sleep until all namespaces with '$USER_PREFIX' prefix are gone [attempt $attempt/$attempts]"
        oc get ns | grep -E "^$USER_PREFIX" >/dev/null 2>&1 || break 1
        sleep "$sleep"
        attempt=$((attempt + 1))
    done
    if [ "$attempt" -le "$attempts" ]; then
        echo " * All the namespaces with '$USER_PREFIX' are gone!"
    else
        echo " * WARNING: Timeout waiting for namespaces with '$USER_PREFIX' to be gone. The following namespaces still exist:"
        oc get ns | grep -E "^$USER_PREFIX"
    fi
}

max_concurrency() {
    local iteration index iteration_index
    # Max length of compliant username is 20 characters. We add "-XXXX-XXXX" suffix for the test users' name so max length of the prefix is 10.
    # See https://github.com/codeready-toolchain/toolchain-common/blob/master/pkg/usersignup/usersignup.go#L16
    if [ ${#USER_PREFIX} -gt 10 ]; then
        echo "Maximal allowed length of user prefix is 10 characters. The '$USER_PREFIX' length of ${#USER_PREFIX} exceeds the limit."
        exit 1
    else
        output="$output_dir/load-tests.max-concurrency.json"
        IFS="," read -r -a maxConcurrencySteps <<<"$(echo "${MAX_CONCURRENCY_STEPS:-1\ 5\ 10\ 25\ 50\ 100\ 150\ 200}" | sed 's/ /,/g')"
        maxThreads=${MAX_THREADS:-10}
        threshold=${THRESHOLD:-300}
        echo '{"startTimestamp":"'"$(date +%FT%T%:z)"'", "maxThreads": '"$maxThreads"', "maxConcurrencySteps": "'"${maxConcurrencySteps[*]}"'", "threshold": '"$threshold"', "maxConcurrencyReached": 0, "computedConcurrency": 0, "workloadKPI": 0, "endTimestamp": "", "errorsTotal": -1}' | jq >"$output"
        iteration=0
        for t in "${maxConcurrencySteps[@]}"; do
            iteration="$((iteration + 1))"
            if (("$t" > "$maxThreads")); then
                break
            fi
            oc login "$OPENSHIFT_API" -u "$OPENSHIFT_USERNAME" -p "$OPENSHIFT_PASSWORD"
            clean_namespaces
            load_test "$t" "$iteration"
            iteration_index="$(printf "%04d" "$iteration")-$(printf "%04d" "$t")"
            jq ".metadata.\"max-concurrency\".iteration = \"$(printf "%04d" "$iteration")\"" "$output_dir/load-tests.json" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output_dir/load-tests.json"
            cp -vf "$output_dir/load-tests.json" "$output_dir/load-tests.max-concurrency.$iteration_index.json"
            cp -vf "$output_dir/load-tests.log" "$output_dir/load-tests.max-concurrency.$iteration_index.log"
            workloadKPI=$(jq '.workloadKPI' "$output_dir/load-tests.json")
            if awk "BEGIN { exit !($workloadKPI > $threshold)}"; then
                echo "The average time a workload took to succeed (${workloadKPI}s) has exceeded a threshold of ${threshold}s with $t threads."
                workloadKPIOld=$(jq '.workloadKPI' "$output")
                threadsOld=$(jq '.maxConcurrencyReached' "$output")
                computedConcurrency=$(python3 -c "import sys; t = float(sys.argv[1]); a = float(sys.argv[2]); b = float(sys.argv[3]); c = float(sys.argv[4]); d = float(sys.argv[5]); print((t - b) / ((d - b) / (c - a)) + a)" "$threshold" "$threadsOld" "$workloadKPIOld" "$t" "$workloadKPI")
                jq ".computedConcurrency = $computedConcurrency" "$output" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output"
                break
            else
                jq ".maxConcurrencyReached = $t" "$output" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output"
                jq ".workloadKPI = $workloadKPI" "$output" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output"
                jq ".computedConcurrency = $t" "$output" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output"
                jq '.endTimestamp = "'"$(date +%FT%T%:z)"'"' "$output" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output"
                errorsTotal=$(jq '.errorsTotal' "$output_dir/load-tests.json")
                jq ".errorsTotal = $errorsTotal" "$output" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output"
            fi
        done
        DRY_RUN=false ./clear.sh "$USER_PREFIX"
    fi
}

max_concurrency

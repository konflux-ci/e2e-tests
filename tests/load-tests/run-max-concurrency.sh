#!/bin/bash
export MY_GITHUB_ORG GITHUB_TOKEN

output_dir="${OUTPUT_DIR:-.}"

load_test() {
    threads=${1:-1}
    index=$(printf "%04d" "$threads")
    ## Enable profiling in Tekton controller
    if [ "${TEKTON_PERF_ENABLE_PROFILING:-}" == "true" ]; then
        echo "Starting CPU profiling with pprof"
        TEKTON_PERF_PROFILE_CPU_PERIOD=${TEKTON_PERF_PROFILE_CPU_PERIOD:-${THRESHOLD:-300}}
        oc exec -n openshift-pipelines "$(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name)" -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/profile?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/cpu-profile.$index.pprof" &
        TEKTON_PROFILER_PID=$!
    fi
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
        --disable-metrics \
        --enable-progress-bars="${ENABLE_PROGRESS_BARS:-false}" \
        --pipeline-skip-initial-checks="${PIPELINE_SKIP_INITIAL_CHECKS:-true}"
    if [ "${TEKTON_PERF_ENABLE_PROFILING:-}" == "true" ]; then
        echo "Waiting for the Tekton controller profiling to finish up to ${TEKTON_PERF_PROFILE_CPU_PERIOD}s"
        wait "$TEKTON_PROFILER_PID"
    fi
}

USER_PREFIX=${USER_PREFIX:-testuser}
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
    echo '{"startTimestamp":"'$(date +%FT%T%:z)'", "maxThreads": '"$maxThreads"', "maxConcurrencySteps": "'"${maxConcurrencySteps[*]}"'", "threshold": '"$threshold"', "maxConcurrencyReached": 0, "endTimestamp": ""}' | jq >"$output"
    for t in "${maxConcurrencySteps[@]}"; do
        if (("$t" > "$maxThreads")); then
            break
        fi
        echo "Deleting resources from previous steps"
        for res in pipelineruns.tekton.dev components.appstudio.redhat.com componentdetectionqueries.appstudio.redhat.com snapshotenvironmentbindings.appstudio.redhat.com applications.appstudio.redhat.com; do
            echo -e " * $res"
            oc get "$res" -A -o json | jq -rc '.items[] | select(.metadata.namespace | startswith("'"$USER_PREFIX"'"))| "oc delete '"$res"' " + .metadata.name + " -n " + .metadata.namespace + ";"' | bash -s
        done
        oc get usersignups.toolchain.dev.openshift.com -A -o name | grep "$USER_PREFIX" | xargs oc delete -n toolchain-host-operator
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
        load_test "$t"
        index=$(printf "%04d" "$t")
        cp -vf "$output_dir/load-tests.json" "$output_dir/load-tests.max-concurrency.$index.json"
        cp -vf "$output_dir/load-tests.log" "$output_dir/load-tests.max-concurrency.$index.log"
        workloadKPI=$(jq '.createApplicationsTimeAvg + .createCDQsTimeAvg + .createComponentsTimeAvg + .integrationTestsRunPipelineSucceededTimeAvg + .runPipelineSucceededTimeAvg + .deploymentSucceededTimeAvg' "$output_dir/load-tests.json")
        if [ "$workloadKPI" -gt "$threshold" ]; then
            echo "The average time a workload took to succeed (${workloadKPI}s) has exceeded a threshold of ${threshold}s with $t threads."
            break
        else
            jq ".maxConcurrencyReached = $t" "$output" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output"
            jq '.endTimestamp = "'$(date +%FT%T%:z)'"' "$output" >"$output_dir/$$.json" && mv -f "$output_dir/$$.json" "$output"
        fi
    done
    DRY_RUN=false ./clear.sh "$USER_PREFIX"
fi

#!/bin/bash
export MY_GITHUB_ORG GITHUB_TOKEN

# Check if the RANDOM_STRING environment variable is declared. If it's declared, include the -r flag when invoking loadtest.go
if [ -n "${RANDOM_PREFIX+x}" ]; then
    RANDOM_PREFIX_FLAG="-r"
else
    RANDOM_PREFIX_FLAG=""
fi

TEKTON_PERF_PROFILE_CPU_PERIOD=${TEKTON_PERF_PROFILE_CPU_PERIOD:-300}

output_dir="${OUTPUT_DIR:-.}"
USER_PREFIX=${USER_PREFIX:-testuser}
# Max length of compliant username is 20 characters. We add -"-XXXXX-XXXX" suffix for the test users' name so max length of the prefix is 9.
# Random string addition will not apply for Openshift-CI load tests
# Therefore: the "-XXXXXX" addition to user prefix will be the PR number for OpenShift Presubmit jobs
# The same addition will be for the 5 chars long random string for non OpenShift-CI load tests
# The last "XXXX" would be for the suffix running index added to the test users in cmd/loadTests.go
# Comment: run-max-concurrency.sh is used for max-concurrency type of Open-Shift-CI jobs and it wasn't changed
# See https://github.com/codeready-toolchain/toolchain-common/blob/master/pkg/usersignup/usersignup.go#L16

# If adding random prefix we can allow only up to 9 characters long user prefix
if [ "${RANDOM_PREFIX_FLAG}" == "-r" ] && [ ${#USER_PREFIX} -gt 9 ]; then
    echo "Maximal allowed length of user prefix is 9 characters. The '$USER_PREFIX' length of ${#USER_PREFIX} exceeds the limit."
    exit 1
# If adding not adding random prefix we can allow only up to 15 characters long user prefix
elif [ "${RANDOM_PREFIX_FLAG}" == "" ] && [ ${#USER_PREFIX} -gt 15 ]; then
    echo "Maximal allowed length of user prefix is 15 characters. The '$USER_PREFIX' length of ${#USER_PREFIX} exceeds the limit."
else
    ## Enable CPU profiling in Tekton
    if [ "${TEKTON_PERF_ENABLE_CPU_PROFILING:-}" == "true" ]; then
        echo "Starting CPU profiling with pprof"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            file="tekton-pipelines-controller.$pod.cpu-profile"
            oc exec -n openshift-pipelines "$p" -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/profile?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/$file.pprof" &
            echo $! >"$output_dir/$file.pid"
        done
        p=$(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name)
        pod="${p##*/}"
        file=tekton-results-watcher.$pod.cpu-profile
        oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/profile?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/$file.pprof" &
        echo $! >"$output_dir/$file.pid"
    fi
    ## Enable memory profiling in Tekton
    if [ "${TEKTON_PERF_ENABLE_MEMORY_PROFILING:-}" == "true" ]; then
        file=tekton-pipelines-controller.memory-profile
        echo "Starting memory profiling of Tekton controller with pprof"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            file="tekton-pipelines-controller.$pod.memory-profile"
            oc exec -n openshift-pipelines "$p" -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/heap?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/$file.pprof" &
            echo $! >"$output_dir/$file.pid"
        done
        echo "Starting memory profiling of Tekton results watcher with pprof"
        for p in $(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name); do
            pod="${p##*/}"
            file=tekton-results-watcher.$pod.memory-profile
            oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/heap?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >"$output_dir/$file.pprof" &
            echo $! >"$output_dir/$file.pid"
        done
    fi
    ## Switch KubeScheduler Debugging on
    if [ -n "$KUBE_SCHEDULER_LOG_LEVEL" ]; then
        echo "Checking KubeScheduler log level"
        if [ "$(oc get KubeScheduler cluster -o jsonpath="{.spec.logLevel}")" == "$KUBE_SCHEDULER_LOG_LEVEL" ]; then
            echo "KubeScheduler log level is already at $KUBE_SCHEDULER_LOG_LEVEL level"
        else
            echo "Setting KubeScheduler log level to $KUBE_SCHEDULER_LOG_LEVEL"
            oc patch KubeScheduler cluster --type=json -p='[{"op": "add", "path": "/spec/logLevel", "value": "'"$KUBE_SCHEDULER_LOG_LEVEL"'"}]'
            echo "Waiting for kube scheduler to start NodeInstallerProgressing"
            oc wait --for=condition=NodeInstallerProgressing kubescheduler/cluster -n openshift-kube-scheduler --timeout=300s
        fi
        echo "Waiting for all kube scheduler pods to finish NodeInstallerProgressing"
        oc wait --for=condition=NodeInstallerProgressing=False kubescheduler/cluster -n openshift-kube-scheduler --timeout=900s
        echo "All kube scheduler pods are now at log level $KUBE_SCHEDULER_LOG_LEVEL, starting to capture logs"
        oc logs -f -n openshift-kube-scheduler --prefix -l app=openshift-kube-scheduler --tail=-1 2>&1 >"$output_dir/openshift-kube-scheduler.log" &
        KUBE_SCHEDULER_LOG_PID=$!
    fi
    ## Run the actual load test
    ## To enable progress bar , add `--enable-progress-bars` in [OPTIONS]
    go run loadtest.go \
        --component-repo "${COMPONENT_REPO:-https://github.com/devfile-samples/devfile-sample-code-with-quarkus}" \
        --component-repo-revision "${COMPONENT_REPO_REVISION:-main}" \
        --build-pipeline-selector-bundle "${BUILD_PIPELINE_SELECTOR_BUNDLE:-}" \
        --username "$USER_PREFIX" \
        --users "${USERS_PER_THREAD:-50}" \
        --test-scenario-git-url "${TEST_SCENARIO_GIT_URL:-https://github.com/konflux-ci/integration-examples.git}" \
        --test-scenario-revision "${TEST_SCENARIO_REVISION:-main}" \
        --test-scenario-path-in-repo "${TEST_SCENARIO_PATH_IN_REPO:-pipelines/integration_resolver_pipeline_pass.yaml}" \
        -w="${WAIT_PIPELINES:-true}" \
        -i="${WAIT_INTEGRATION_TESTS:-true}" \
        -d="${WAIT_DEPLOYMENTS:-true}" \
        -l \
        -o "$output_dir" \
        -t "${THREADS:-1}" \
        $RANDOM_PREFIX_FLAG \
        --disable-metrics="${DISABLE_METRICS:-false}" \
        --pushgateway-url "${PUSHGATEWAY_URL:-rhtapqe.com}" \
        --enable-progress-bars="${ENABLE_PROGRESS_BARS:-false}" \
        --pipeline-skip-initial-checks="${PIPELINE_SKIP_INITIAL_CHECKS:-true}"

    DRY_RUN=false ./clear.sh "$USER_PREFIX"

    if [ "${TEKTON_PERF_ENABLE_CPU_PROFILING:-}" == "true" ] || [ "${TEKTON_PERF_ENABLE_MEMORY_PROFILING:-}" == "true" ]; then
        echo "Waiting for the Tekton profiling to finish up to ${TEKTON_PERF_PROFILE_CPU_PERIOD}s"
        for pid_file in $(find $output_dir -name 'tekton*.pid'); do
            wait "$(cat "$pid_file")"
            rm -rvf "$pid_file"
        done
        echo "Getting Tekton controller goroutine dump"
        for p in $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name); do
            pod="${p##*/}"
            for i in 0 1 2; do
                file="tekton-pipelines-controller.$pod.goroutine-dump-$i"
                oc exec -n tekton-results "$p" -- bash -c "curl -SsL localhost:8008/debug/pprof/goroutine?debug=$i | base64" | base64 -d >"$output_dir/$file.pprof"
            done
        done
        echo "Getting Tekton results watcher goroutine dump"
        for p in $(oc get pods -n tekton-results -l app.kubernetes.io/name=tekton-results-watcher -o name); do
            pod="${p##*/}"
            for i in 0 1 2; do
                file="tekton-results-watcher.$pod.goroutine-dump-$i"
                oc exec -n tekton-results "$p" -c watcher -- bash -c "curl -SsL localhost:8008/debug/pprof/goroutine?debug=$i | base64" | base64 -d >"$output_dir/$file.pprof"
            done
        done
    fi
    if [ -n "$KUBE_SCHEDULER_LOG_LEVEL" ]; then
        echo "Killing kube collector log collector"
        kill "$KUBE_SCHEDULER_LOG_PID"
    fi
fi

#!/bin/bash
export MY_GITHUB_ORG GITHUB_TOKEN

USER_PREFIX=${USER_PREFIX:-testuser}
# Max length of compliant username is 20 characters. We add "-XXXX" suffix for the test users' name so max length of the prefix is 15.
# See https://github.com/codeready-toolchain/toolchain-common/blob/master/pkg/usersignup/usersignup.go#L16
if [ ${#USER_PREFIX} -gt 15 ]; then
    echo "Maximal allowed length of user prefix is 15 characters. The '$USER_PREFIX' length of ${#USER_PREFIX} exceeds the limit."
    exit 1
else
    ## Enable profiling in Tekton controller
    if [ "${TEKTON_PERF_ENABLE_PROFILING:-}" == "true" ]; then
        echo "Starting CPU profiling with pprof"
        TEKTON_PERF_PROFILE_CPU_PERIOD=${TEKTON_PERF_PROFILE_CPU_PERIOD:-300}
        oc exec -n openshift-pipelines $(oc get pods -n openshift-pipelines -l app=tekton-pipelines-controller -o name) -- bash -c "curl -SsL --max-time $((TEKTON_PERF_PROFILE_CPU_PERIOD + 10)) localhost:8008/debug/pprof/profile?seconds=${TEKTON_PERF_PROFILE_CPU_PERIOD} | base64" | base64 -d >cpu-profile.pprof &
        TEKTON_PROFILER_PID=$!
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
        oc logs -f -n openshift-kube-scheduler --prefix -l app=openshift-kube-scheduler --tail=-1 2>&1 >openshift-kube-scheduler.log &
        KUBE_SCHEDULER_LOG_PID=$!
    fi
    ## Run the actual load test
    go run loadtest.go \
        --component-repo "${COMPONENT_REPO:-https://github.com/devfile-samples/devfile-sample-code-with-quarkus}" \
        --username "$USER_PREFIX" \
        --users "${USERS_PER_THREAD:-50}" \
        --test-scenario-git-url "${TEST_SCENARIO_GIT_URL:-https://github.com/redhat-appstudio/integration-examples.git}" \
        --test-scenario-revision "${TEST_SCENARIO_REVISION:-main}" \
        --test-scenario-path-in-repo "${TEST_SCENARIO_PATH_IN_REPO:-pipelines/integration_resolver_pipeline_pass.yaml}" \
        -w="${WAIT_PIPELINES:-true}" \
        -i="${WAIT_INTEGRATION_TESTS:-true}" \
        -d="${WAIT_DEPLOYMENTS:-true}" \
        -l \
        -t "${THREADS:-1}" \
        --disable-metrics \
        --pipeline-skip-initial-checks="${PIPELINE_SKIP_INITIAL_CHECKS:-true}"

    DRY_RUN=false ./clear.sh "$USER_PREFIX"

    if [ "${TEKTON_PERF_ENABLE_PROFILING:-}" == "true" ]; then
        echo "Waiting for the Tekton controller profiling to finish up to ${TEKTON_PERF_PROFILE_CPU_PERIOD}s"
        wait "$TEKTON_PROFILER_PID"
    fi
    if [ -n "$KUBE_SCHEDULER_LOG_LEVEL" ]; then
        echo "Killing kube collector log collector"
        kill "$KUBE_SCHEDULER_LOG_PID"
    fi
fi

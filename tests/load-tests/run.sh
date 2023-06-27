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

    go run loadtest.go \
        --component-repo "${COMPONENT_REPO:-https://github.com/devfile-samples/devfile-sample-code-with-quarkus}" \
        --username "$USER_PREFIX" \
        --users "${USERS_PER_THREAD:-50}" \
        -w \
        -l \
        -t "${THREADS:-1}" \
        --disable-metrics \
        --pipeline-skip-initial-checks="${PIPELINE_SKIP_INITIAL_CHECKS:-true}" &&
        DRY_RUN=false ./clear.sh "$USER_PREFIX"

    if [ "${TEKTON_PERF_ENABLE_PROFILING:-}" == "true" ]; then
        echo "Waiting for the Tekton controller profiling to finish up to ${TEKTON_PERF_PROFILE_CPU_PERIOD}s"
        wait "$TEKTON_PROFILER_PID"
    fi
fi

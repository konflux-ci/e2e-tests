#!/bin/bash
export DOCKER_CONFIG_JSON MY_GITHUB_ORG GITHUB_TOKEN
DOCKER_CONFIG_JSON=${DOCKER_CONFIG_JSON:-}

load_test() {
    threads=${1:-1}
    go run loadtest.go \
        --component-repo "${COMPONENT_REPO:-https://github.com/nodeshift-starters/devfile-sample.git}" \
        --username "$USER_PREFIX-$(printf "%04d" "$threads")" \
        --users 1 \
        -w \
        -l \
        -t "$threads" \
        --disable-metrics \
        --pipeline-skip-initial-checks="${PIPELINE_SKIP_INITIAL_CHECKS:-true}"
}

if [ -z ${DOCKER_CONFIG_JSON+x} ]; then
    echo "env DOCKER_CONFIG_JSON need to be defined"
    exit 1
else echo "DOCKER_CONFIG_JSON is set"; fi

USER_PREFIX=${USER_PREFIX:-testuser}
# Max length of compliant username is 20 characters. We add "-XXXX-XXXX" suffix for the test users' name so max length of the prefix is 10.
# See https://github.com/codeready-toolchain/toolchain-common/blob/master/pkg/usersignup/usersignup.go#L16
if [ ${#USER_PREFIX} -gt 10 ]; then
    echo "Maximal allowed length of user prefix is 10 characters. The '$USER_PREFIX' length of ${#USER_PREFIX} exceeds the limit."
    exit 1
else
    output=load-tests.max-concurrency.json
    maxThreads=${MAX_THREADS:-10}
    threshold=${THRESHOLD:-300}
    echo '{"maxThreads": '"$maxThreads"', "threshold": '"$threshold"', "maxConcurrencyReached": 0, "steps": []}' | jq >"$output"
    for t in $(seq 1 "${MAX_THREADS:-10}"); do
        oc get usersignups.toolchain.dev.openshift.com -A -o name | grep "$USER_PREFIX" | xargs oc delete -n toolchain-host-operator
        while true; do
            echo "Waiting until all namespaces with '$USER_PREFIX' prefix are gone..."
            oc get ns | grep "$USER_PREFIX" >/dev/null 2>&1 || break 1
            sleep 5s
        done
        load_test "$t"
        jq --slurpfile result "load-tests.json" '.steps += $result' "$output" >$$.json && mv -f $$.json "$output"
        mv -f load-tests.log "load-tests.max-concurrency.$(printf "%04d" "$t").log"
        pipelineRunThresholdExceeded=$(jq -rc ".runPipelineSucceededTimeMax > $threshold" load-tests.json)
        pipelineRunKPI=$(jq -rc ".runPipelineSucceededTimeMax" load-tests.json)
        if [ "$pipelineRunThresholdExceeded" = "true" ]; then
            echo "The maximal time a pipeline run took to succeed (${pipelineRunKPI}s) has exceeded a threshold of ${threshold}s with $t threads."
            break
        else
            jq ".maxConcurrencyReached = $t" "$output" >$$.json && mv -f $$.json "$output"
        fi
    done
    DRY_RUN=false ./clear.sh "$USER_PREFIX"
fi

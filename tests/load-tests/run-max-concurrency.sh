#!/bin/bash
export MY_GITHUB_ORG GITHUB_TOKEN

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
    echo '{"maxThreads": '"$maxThreads"', "threshold": '"$threshold"', "maxConcurrencyReached": 0}' | jq >"$output"
    for t in $(seq 1 "${MAX_THREADS:-10}"); do
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
        cp -vf load-tests.json "load-tests.max-concurrency.$index.json"
        cp -vf load-tests.log "load-tests.max-concurrency.$index.log"
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

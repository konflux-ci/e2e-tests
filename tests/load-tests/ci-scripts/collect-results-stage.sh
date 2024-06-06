#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

source "$( dirname $0 )/utils.sh"

ARTIFACT_DIR="${1:-.}"
CONCURRENCY="${2:-1}"

login_log_stub=$ARTIFACT_DIR/collected-oc_login

application_stub=$ARTIFACT_DIR/collected-applications.appstudio.redhat.com
component_stub=$ARTIFACT_DIR/collected-components.appstudio.redhat.com
pipelinerun_stub=$ARTIFACT_DIR/collected-pipelineruns.tekton.dev
taskrun_stub=$ARTIFACT_DIR/collected-taskruns.tekton.dev
pod_stub=$ARTIFACT_DIR/collected-pods
node_stub=$ARTIFACT_DIR/collected-nodes

if ! [ -r users.json ]; then
    echo "ERROR: Missing file with user creds"
fi

for uid in $( seq 1 $CONCURRENCY ); do
    username="test-rhtap-$uid"
    offline_token=$( cat users.json | jq --raw-output '.[] | select(.username == "'$username'").token' )
    api_server=$( cat users.json | jq --raw-output '.[] | select(.username == "'$username'").apiurl' )
    sso_server=$( cat users.json | jq --raw-output '.[] | select(.username == "'$username'").ssourl' )
    access_token=$( curl \
                      --silent \
                      --header "Accept: application/json" \
                      --header "Content-Type: application/x-www-form-urlencoded" \
                      --data-urlencode "grant_type=refresh_token" \
                      --data-urlencode "client_id=cloud-services" \
                      --data-urlencode "refresh_token=${offline_token}" \
                      "${sso_server}" \
                    | jq --raw-output ".access_token" )
    login_log="${login_log_stub}-${username}.log"
    echo "Logging in as $username..."
    if ! oc login --token="$access_token" --server="$api_server" &>$login_log; then
        echo "ERROR: Login as $username failed:"
        cat "$login_log"
        continue
    fi
    tenant="${username}-tenant"

    ## Application info
    echo "Collecting Application timestamps..."
    collect_application "-n ${tenant}" "$application_stub-$tenant"

    ## Component info
    echo "Collecting Component timestamps..."
    collect_component "-n ${tenant}" "$component_stub-$tenant"

    ### PipelineRun info
    #echo "Collecting PipelineRun timestamps..."
    #collect_pipelinerun "-n ${tenant}" "$pipelinerun_stub-$tenant"

    ### TaskRun info
    #echo "Collecting TaskRun timestamps..."
    #collect_taskrun "-n ${tenant}" "$taskrun_stub-$tenant"

    ### Pods info
    #echo "Collecting node specs..."
    #collect_pods "-n ${tenant}" "$pod_stub-$tenant"
done

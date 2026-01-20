#!/bin/bash

set -eu

if [[ -z ${QUAY_TOKEN:-} ]]; then
    echo "ERROR: Please export QUAY_TOKEN with 'Administer Repositories' permission"   # See https://docs.quay.io/api/
    exit 1
fi

NAMESPACE="stonesoup_perfscale"

deleted_counter=0
failed_counter=0
pages_counter=1
while true; do
    # See https://docs.quay.io/api/swagger/#!/repository/listRepos
    response="$( curl --silent -H "Authorization: Bearer $QUAY_TOKEN" "https://quay.io/api/v1/repository?namespace=$NAMESPACE" | jq -r '.' )"

    repositories="$( echo "$response" | jq -r '.repositories[] | .name' )"
    for repo in $repositories; do
        echo "Page $pages_counter repo $repo"
        http_code="$( curl --silent -X DELETE -w "%{http_code}" -H "Authorization: Bearer $QUAY_TOKEN" "https://quay.io/api/v1/repository/$NAMESPACE/$repo" )"
        if [[ $http_code == "204" ]]; then
            (( deleted_counter += 1 ))
        else
            echo "ERROR: Failed to delete $repo: $http_code"
            (( failed_counter += 1 ))
        fi
    done

    next_page="$( echo "$response" | jq '.next_page' )"
    if [[ -z $next_page || $next_page == "null" ]]; then
        echo "No more pages"
        break
    fi

    (( pages_counter += 1 ))
done

echo "Processed $pages_counter oages of results, deleted $deleted_counter and failed to delete $failed_counter repositories."

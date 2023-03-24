#!/bin/bash

set -xe
set -o pipefail

USER_PREFIX=${1:-testuser}
COUNTER=0

while true; do
    REPO_LIST=$( curl --fail -s https://api.github.com/users/${MY_GITHUB_ORG}/repos\?per_page\=100 | jq -r '.[]|select(.name | startswith("'${USER_PREFIX}'"))' | jq --raw-output '.name' )

    if [[ ${#REPO_LIST} == 0 ]]; then
        break
    fi

    for REPO in $REPO_LIST; do
        curl --fail-with-body \
            -X DELETE \
            -H "Accept: application/vnd.github.v3+json" \
            -H "Authorization: token $GITHUB_TOKEN" \
            https://api.github.com/repos/${MY_GITHUB_ORG}/${REPO}
        let COUNTER+=1
    done
done

echo "Deleted $COUNTER repos"

#!/bin/bash -x
USER_PREFIX=${1:-testuser}
REPO_LIST=$(curl -s https://api.github.com/users/${MY_GITHUB_ORG}/repos\?per_page\=1000 | jq -r '.[]|select(.name | startswith("'${USER_PREFIX}'"))' | jq --raw-output '.name')

for REPO in $REPO_LIST
do
	curl \
    -X DELETE \
    -H "Accept: application/vnd.github.v3+json" \
    -H "Authorization: token $GITHUB_TOKEN" \
    https://api.github.com/repos/${MY_GITHUB_ORG}/${REPO}
done

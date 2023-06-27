#!/bin/bash
USER_PREFIX=${1:-testuser}

response_headers=$(mktemp)
repo_list=$(mktemp)

curl_args='-s -H "Accept: application/vnd.github+json" -H "Authorization: token '$GITHUB_TOKEN'" -H "X-GitHub-Api-Version: 2022-11-28" -D '$response_headers

echo -n "Collecting list of repos with prefix '${USER_PREFIX}' in '${MY_GITHUB_ORG}' organization to delete"
page=1
while true; do
    echo -n "."
    echo "$curl_args" "https://api.github.com/orgs/${MY_GITHUB_ORG}/repos?per_page=100&page=$page" | xargs curl | jq -r '.[] | select(.name | startswith("'"$USER_PREFIX-"'")).name' >>"$repo_list"

    if ! grep -q 'rel="next"' "$response_headers"; then
        break
    fi

    ((page++))
done

DRY_RUN=${DRY_RUN:-true}
echo " Found $(wc -l <"$repo_list") repos"
while read -r repo; do
    if [ "$DRY_RUN" == "false" ]; then
        echo "Deleting repo $MY_GITHUB_ORG/$repo"
        echo "$curl_args" -X DELETE "https://api.github.com/repos/${MY_GITHUB_ORG}/$repo" | xargs curl
    else
        echo "[DRY-RUN] Would have deleted repo $MY_GITHUB_ORG/$repo"
    fi
done <"$repo_list"

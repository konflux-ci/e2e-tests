REPO_LIST=$(curl -s https://api.github.com/users/app-studio-test/repos\?per_page\=1000 | jq -r '.[]|select(.name | startswith("testuser"))' | jq --raw-output '.name')

for REPO in $REPO_LIST
do
	curl \
    -X DELETE \
    -H "Accept: application/vnd.github.v3+json" \
    -H "Authorization: token $GITHUB_TOKEN" \
    https://api.github.com/repos/app-studio-test/${REPO}
done

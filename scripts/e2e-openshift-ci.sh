#!/bin/bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

if [ -z "${OPENSHIFT_CI}" ]; then
    echo "[ERROR] The script is not running in openshift ci"
    exit 1
fi

mkdir -p tmp/

export ROOT_E2E="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..
export WORKSPACE=${WORKSPACE:-${ROOT_E2E}}
export E2E_CLONE_BRANCH="main"
export E2E_REPO_LINK="https://github.com/redhat-appstudio/e2e-tests.git"
export AUTHOR_E2E_BRANCH=""

function exists_public_github_repo() {
    local pr_author=$1

    if curl -fsS "https://api.github.com/repos/${pr_author}/e2e-tests" >/dev/null; then
        echo -e "[INFO] The GitHub repo ${pr_author}/e2e-tests exists."
        return 0
    else
        echo -e "[ERROR] No GitHub repo ${pr_author}/e2e-tests found."
        return 1
    fi
}

function pairPullRequests() {
    # Example: CLONEREFS_OPTIONS={"src_root":"/go","log":"/dev/null","git_user_name":"ci-robot","git_user_email":"ci-robot@openshift.io","refs":[{"org":"redhat-appstudio","repo":"application-service","repo_link":"https://github.com/redhat-appstudio/application-service","base_ref":"main","base_sha":"75a4c79e49ab5c1a4c15d844256d1e4419da63e3","base_link":"https://github.com/redhat-appstudio/application-service/commit/75a4c79e49ab5c1a4c15d844256d1e4419da63e3","pulls":[{"number":91,"author":"flacatus","sha":"47b9fe555e27cc65c5ebfcf51c2d26a036fab235","link":"https://github.com/redhat-appstudio/application-service/pull/91","commit_link":"https://github.com/redhat-appstudio/application-service/pull/91/commits/47b9fe555e27cc65c5ebfcf51c2d26a036fab235","author_link":"https://github.com/flacatus"}]}],"fail":true}
    # Checking if CLONEREFS_OPTIONS openshift ci env exists and extract PR information to pair the PR
    # Pairing the PR with the e2e tests: Check the PR branch with the author of PR fork of e2e-tests. For example user Bill open a PR in application-service, the script check if 
    # exists a branch in the e2e-tests with the same name of PR branch.
    if [[ -n ${CLONEREFS_OPTIONS} ]]; then
        AUTHOR=$(jq -r '.refs[0].pulls[0].author' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
        AUTHOR_LINK=$(jq -r '.refs[0].pulls[0].author_link' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
        GITHUB_ORGANIZATION=$(jq -r '.refs[0].org' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')
        GITHUB_REPO=$(jq -r '.refs[0].repo' <<< ${CLONEREFS_OPTIONS} | tr -d '[:space:]')

        PR_BRANCH_REF=$(curl -H "Authorization: token ${GITHUB_TOKEN}" https://api.github.com/repos/"${GITHUB_ORGANIZATION}"/"${GITHUB_REPO}"/pulls/"${PULL_NUMBER}" | jq --raw-output .head.ref)

        if exists_public_github_repo "${AUTHOR}/e2e-tests"; then
            AUTHOR_E2E_BRANCH=$(curl -H "Authorization: token ${GITHUB_TOKEN}" https://api.github.com/repos/"${AUTHOR}"/e2e-tests/branches | jq '.[] | select(.name=="'${PR_BRANCH_REF}'")')
        fi

        if [ -z "${AUTHOR_E2E_BRANCH}" ]; then
            E2E_REPO_LINK="https://github.com/redhat-appstudio/e2e-tests.git"
        else
            E2E_CLONE_BRANCH=${PR_BRANCH_REF}
            E2E_REPO_LINK="${AUTHOR_LINK}/e2e-tests.git"
        fi
    fi
}

pairPullRequests
echo "[INFO] Cloning tests from branch ${PR_BRANCH_REF} repository ${E2E_REPO_LINK}"
git clone -b "${E2E_CLONE_BRANCH}" "${E2E_REPO_LINK}" "$WORKSPACE"/tmp/e2e-tests

cd "$WORKSPACE"/tmp/e2e-tests
make build
chmod 755 "$WORKSPACE"/tmp/e2e-tests/bin/e2e-appstudio
cd "$WORKSPACE"

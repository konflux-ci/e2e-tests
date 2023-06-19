#!/bin/bash
export MY_GITHUB_ORG GITHUB_TOKEN

USER_PREFIX=${USER_PREFIX:-testuser}
# Max length of compliant username is 20 characters. We add "-XXXX" suffix for the test users' name so max length of the prefix is 15.
# See https://github.com/codeready-toolchain/toolchain-common/blob/master/pkg/usersignup/usersignup.go#L16
if [ ${#USER_PREFIX} -gt 15 ]; then
    echo "Maximal allowed length of user prefix is 15 characters. The '$USER_PREFIX' length of ${#USER_PREFIX} exceeds the limit."
    exit 1
else
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
fi

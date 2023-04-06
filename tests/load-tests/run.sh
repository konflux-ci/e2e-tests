#!/bin/bash
export DOCKER_CONFIG_JSON=${DOCKER_CONFIG_JSON:-}

if [ -z ${DOCKER_CONFIG_JSON+x} ]; then
    echo "env DOCKER_CONFIG_JSON need to be defined"
    exit 1
else echo "DOCKER_CONFIG_JSON is set"; fi

USER_PREFIX=${USER_PREFIX:-testuser}
go run loadtest.go --username "$USER_PREFIX" --users "${USERS_PER_THREAD:-50}" --batch "${USERS_BATCH:-10}" -w -l -t "${THREADS:-1}" --disable-metrics &&
    DRY_RUN=false ./clear.sh "$USER_PREFIX"

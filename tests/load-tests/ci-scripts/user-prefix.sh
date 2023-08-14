#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

export USER_PREFIX

if [ -n "${PULL_NUMBER:-}" ]; then
    new_prefix="${USER_PREFIX}-${PULL_NUMBER}"
    echo "To avoid naming collisions adding PR number to USER_PREFIX: '${USER_PREFIX}' -> '${new_prefix}'"
    USER_PREFIX="$new_prefix"
fi

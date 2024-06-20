# This is supposed to be sourced

export USER_PREFIX="${USER_PREFIX:-testuser}"

if [ -n "${PULL_NUMBER:-}" ]; then
    new_prefix="${USER_PREFIX}-${PULL_NUMBER}"
    echo "To avoid naming collisions adding PR number to USER_PREFIX: '${USER_PREFIX}' -> '${new_prefix}'"
    USER_PREFIX="$new_prefix"
fi

go run loadtest.go \
        --component-repo "${COMPONENT_REPO:-https://github.com/devfile-samples/devfile-sample-code-with-quarkus}" \
        --users "${USERS_PER_THREAD:-3}" \
        -s \
        -w \
        -l \
        -t "${THREADS:-2}" \
        --disable-metrics \
        --pipeline-skip-initial-checks="${PIPELINE_SKIP_INITIAL_CHECKS:-true}"
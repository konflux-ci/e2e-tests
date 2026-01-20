Various configurations needed for our tests
===========================================

Horreum schema
--------------

Defines labels we are interested in.

Link to the schema in Horreum: https://horreum.corp.redhat.com/schema/169

To import modified versions, follow this guide: https://horreum.hyperfoil.io/docs/tasks/import-export/#import-or-export-using-the-api

To list existing label names:

    jq -r '.labels[] | [.name, .extractors[0].jsonpath] | @tsv' ci-scripts/config/horreum-schema.json | column --separator "	" --table

To delete a label by it's name:

    label_del="__results_durations_stats_taskruns__build_calculate_deps__passed_duration_mean"
    jq 'del(.labels[] | select(.name == "'"$label_del"'"))' ci-scripts/config/horreum-schema.json > tmp-$$.json && mv tmp-$$.json ci-scripts/config/horreum-schema.json

To add a label given it's JSONPath expression:

    jsonpath_add='$.results.durations.stats.taskruns."build/calculate-deps".passed.duration.mean'
    label_add="$( echo "$jsonpath_add" | sed 's/[^a-zA-Z0-9]/_/g' )"
    jq '.labels += [{"access": "PUBLIC", "owner": "hybrid-cloud-experience-perfscale-team", "name": "'"$label_add"'", "extractors": [{"name": "'"$label_add"'", "jsonpath": "'"$( echo "$jsonpath_add" | sed 's/"/\\\"/g' )"'", "isarray": false}], "_function": "", "filtering": false, "metrics": true, "schemaId": 169}]' ci-scripts/config/horreum-schema.json > tmp-$$.json && mv tmp-$$.json ci-scripts/config/horreum-schema.json

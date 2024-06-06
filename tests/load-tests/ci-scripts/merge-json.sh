#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

# Check if jq is installed
if ! command -v jq &>/dev/null; then
    echo "jq not found. Please Install it"
    exit 1
fi

# Loop through all the variables and save them to files
n=1
for var in $@; do
    echo "$var" | base64 --decode >"pre_${n}.json"
    let n++
done

# Merge all the pre_N.json files into users.json
jq -s '.[]' pre_*.json | jq -s 'add' > users.json

# Remove the temporary pre_N.json files
rm pre_*.json

echo "Decoded JSON data merged and stored in users.json."

#!/bin/bash

# Function to check if jq is installed and install it if not (for Ubuntu-based systems)
check_and_install_jq() {
    if ! command -v jq &>/dev/null; then
        echo "jq not found. Installing jq..."
        sudo apt-get update
        sudo apt-get install -y jq
    fi
}

# Function to merge JSON strings into one JSON array
merge_json_strings() {
    local merged_json_array=$(jq -s '.' <<< "$@")
    echo "$merged_json_array"
}

# Check and install jq if needed
check_and_install_jq

# Get the output file path from the first argument
output_file="${1}"
shift

# Merge JSON strings into one JSON array
merged_json=$(merge_json_strings "$@")

echo $merged_json

# Store the merged JSON array in the output file
echo "$merged_json" > "$output_file"

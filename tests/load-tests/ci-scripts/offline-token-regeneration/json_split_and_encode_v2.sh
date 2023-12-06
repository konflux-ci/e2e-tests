#!/bin/bash

# Check for input file argument
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <input_file> <output_file>"
    exit 1
fi

input_file="$1"
parts_count="$2" # Number of parts to split into

# Function to split JSON, encode in base64, and save to separate files
split_encode_and_save_json() {
    local input_file="$1"
    local parts_count="$2"

    # Calculate the number of lines per part
    local total_lines=$(jq '.creds | length' < "$input_file")
    local lines_per_part=$(( (total_lines + parts_count - 1) / parts_count ))

    # Split the JSON array, encode each part, and save to files
    for ((i = 0; i < parts_count; i++)); do
        local start=$(( i * lines_per_part ))
        local end=$(( start + lines_per_part - 1 ))

        # Extract part of the JSON array, encode it, and save to a file
        # jq ".creds | .[$start:$end]" < "$input_file" | base64 -w 0 > "part_$(($i + 1)).encoded"
input_file="$1"
        # jq "{ \"creds\": .creds | .[$start:$end] }" < "$input_file" > "part_$(($i + 1)).json"
        jq "{ \"creds\": .creds | .[$start:$end] }" < "$input_file" | base64 -w 0 > "part_$(($i + 1)).encoded"
    done
}

split_encode_and_save_json "$input_file" "$parts_count"

echo "Splitting and encoding completed. Part files are created."

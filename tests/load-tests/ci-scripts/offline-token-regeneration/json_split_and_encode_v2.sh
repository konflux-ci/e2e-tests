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

    # Calculate the total number of records in the JSON array
    local total_lines=$(jq '. | length' < "$input_file")

    # Calculate the number of records per part, ensuring it rounds up
    # This ensures that each part will have an equal number of records
    # with the last part possibly having fewer.
    local lines_per_part=$(( (total_lines + parts_count - 1) / parts_count ))

    # Split the JSON array, encode each part, and save to files
    for ((i = 0; i < parts_count; i++)); do
        # Calculate the start and end indices for each part
        local start=$(( i * lines_per_part ))
        local end=$(( start + lines_per_part ))

        # Adjust end index if it exceeds the total number of records
        # This is necessary for the last part if total_records is not
        # perfectly divisible by parts_count.
        if [ $end -gt $total_lines ]; then
            end=$total_lines
        fi

        # Extract part of the JSON array and save base64 encoded to a file
        jq ".[$start:$end]" < "$input_file" | base64 -w 0 > "part_$(($i + 1)).encoded"
    done
}

split_encode_and_save_json "$input_file" "$parts_count"

echo "Splitting and encoding completed. Part files are created."

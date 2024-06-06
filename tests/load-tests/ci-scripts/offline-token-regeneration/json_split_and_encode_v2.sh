#!/bin/bash

# Split JSON with array in file <input_file> to smaller <parts_count> partial
# files. Resulting files are base64 encoded.

# Check for input file argument
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <input_file> <parts_count>"
    exit 1
fi

input_file="$1"
parts_count="$2" # Number of parts to split into

# Calculate the total number of records in the JSON array
total_lines=$(jq '. | length' < "$input_file")

# Calculate the number of records per part, ensuring it rounds up.
# This ensures that each part will have an equal number of records
# with the last part possibly having fewer.
lines_per_part=$(( (total_lines + parts_count - 1) / parts_count ))

# Split the JSON array, encode each part, and save to files
for ((i = 0; i < parts_count; i++)); do
    # Calculate the start and end indices for each part
    start=$(( i * lines_per_part ))
    end=$(( start + lines_per_part ))

    # Adjust end index if it exceeds the total number of records
    # This is necessary for the last part if total_records is not
    # perfectly divisible by parts_count.
    if [ $end -gt $total_lines ]; then
        end=$total_lines
    fi

    # Extract part of the JSON array and save base64 encoded to a file
    jq ".[$start:$end]" < "$input_file" | base64 -w 0 > "part_$(($i + 1)).encoded"
done

echo "Splitting and encoding completed. Part files are created."

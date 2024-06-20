#!/bin/bash

# Check for input file argument
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <input_file> <output_file>"
    exit 1
fi

input_file="$1"
output_file="$2"
ssourl_actual_value="https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"

# Process the file and create line-delimited JSON
while read -r username password token ssourl apiurl verified; do
    echo "{\"username\":\"$username\", \"password\":\"$password\", \"token\":\"$token\", \"ssourl\":\"$ssourl\", \"apiurl\":\"$apiurl\", \"verified\":$verified}"
done < "$input_file" > temp.json

# Use jq to wrap the objects into an array, update ssourl and output to the final file
jq --arg ssourlVal "$ssourl_actual_value" '[ .[] | .ssourl = $ssourlVal ]' -s temp.json > "$output_file"

# Clean up the temporary file
rm temp.json

echo "Conversion completed. JSON output is in $output_file"

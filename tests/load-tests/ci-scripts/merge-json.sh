#!/bin/bash

# Function to check if jq is installed and install it if not (for Ubuntu-based systems)
check_and_install_jq() {
    if ! command -v jq &>/dev/null; then
        echo "jq not found. Please Install it"
        exit 1
    fi
}

# make a dummy kube config
mkdir -p ~/.kube
cat <<EOF > ~/.kube/config
apiVersion: v1
clusters:
- cluster:
    server: YOUR_KUBE_API_SERVER_URL
  name: YOUR_CLUSTER_NAME
contexts:
- context:
    cluster: YOUR_CLUSTER_NAME
    user: YOUR_USER_NAME
  name: YOUR_CONTEXT_NAME
current-context: YOUR_CONTEXT_NAME
kind: Config
preferences: {}
users:
- name: YOUR_USER_NAME
  user:
    token: YOUR_KUBE_API_TOKEN
EOF



# Function to decode and save JSON data from variable to pre_N.json
decode_and_save_json() {
    local variable_name="$1"
    local file_name="pre_${variable_name#*_}.json"
    echo "${!variable_name}" | base64 --decode | jq '.' > "$file_name"
}

# Get the total number of parameters passed to the script (N)
N=$#

# Loop through all the variables
for ((i = 1; i <= N; i++)); do
    var_name="STAGING_USERS_$i"
    decode_and_save_json "$var_name"
done

# Merge all the pre_N.json files into output.json
jq '[.[].creds] | add' pre_*.json > output.json


# Optionally, you can remove the temporary pre_N.json files
# Uncomment the next line to delete the files
rm pre_*.json

echo "Decoded JSON data merged and stored in output.json."

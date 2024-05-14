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
    local file_name="pre_${1}.json"
    cat "$f" | base64 --decode > "$file_name"
}

# Loop through all the variables
for f in $@; do
    decode_and_save_json "$f"
done

# Merge all the pre_N.json files into users.json
jq -s '.[]' pre_*.json | jq -s 'add' > users.json

# Optionally, you can remove the temporary pre_N.json files
# Uncomment the next line to delete the files
rm pre_*.json

echo "Decoded JSON data merged and stored in users.json."

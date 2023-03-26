#!/bin/bash

# Array of default namespaces
default_namespaces=(
    "application-service"
    "build-service"
    "enterprise-contract-service"
    "gitops-service-argocd"
    "integration-service"
    "internal-services"
    "jvm-build-service" 
    "openshift-service-ca"
    "openshift-service-ca-operator"
    "release-service")

# Read namespaces from a file if it exists, otherwise use the default namespaces array
namespaces_file="namespaces.txt"
if [ -f "${namespaces_file}" ]; then
  mapfile -t namespaces < "${namespaces_file}"
else
  namespaces=("${default_namespaces[@]}")
fi


# Log directory configuration
# Format:  ./collect-openshift-pod-logs.sh logs-${USER}-$(date +%Y-%m-%d)  

# if no parameter given, a default will be used for the log_dir
if [ -z "$1" ]; then
  # Set default log directory
  log_dir="logs-${USER}-$(date +%Y-%m-%d)"
else
  # Use log directory provided as script argument
  log_dir="$1"
fi


# Create log directory if it doesn't exist
mkdir -p "${log_dir}"

# Function to collect logs from existing pods
collect_logs_from_existing_pods() {
  for namespace in "${namespaces[@]}"; do
    existing_pods=$(kubectl get pods -n "${namespace}" --no-headers -o custom-columns=":metadata.name")
    for pod_name in $existing_pods; do
      collect_logs "${namespace}" "${pod_name}"
    done
  done
}


# Function to tail logs of a created or existing pod in the background and store it in a distinct file
collect_logs() {
  local namespace=$1
  local pod_name=$2
  local log_file="${log_dir}/${namespace}-${pod_name}.log"

  echo "Collecting logs for pod ${pod_name} in namespace ${namespace} ..."
  kubectl logs -n "${namespace}" -F "${pod_name}" >> "${log_file}" 2>/dev/null &
}

# Function to list the log file of a deleted pod, send it to the remote store, and delete it if empty
list_log_file() {
  local namespace=$1
  local pod_name=$2
  local log_file="${log_dir}/${namespace}-${pod_name}.log"

  # If the file exists then process it..
  if [ -e "${log_file}" ]; then
    # Check if the log file is empty and delete it if so
    if [ ! -s "${log_file}" ]; then
      echo "Log file ${log_file} is empty. Deleting..."
      rm "${log_file}"
    else
      # Send the log file to the remote store
      echo "Pod ${pod_name} in namespace ${namespace} deleted. Logfile: ${log_file}"
    fi
  fi
}


# Start collecting new data from all current namespace pods
collect_logs_from_existing_pods


# Function to clean up and exit the script
cleanup_and_exit() {
  echo "Received SIGINT. Cleaning up and exiting..."
  kill $(jobs -p) 2>/dev/null
  rm -f "${log_dir}"/*.log
  exit 0
}

# Trap SIGINT (Ctrl+C) and call cleanup_and_exit function
trap 'cleanup_and_exit' SIGINT



#  Main loop to watch for created and deleted pods
for namespace in "${namespaces[@]}"; do
  kubectl get pods --watch --namespace "${namespace}" --output-watch-events --output jsonpath='{.type} {.object.metadata.name}{"\n"}' 2>/dev/null | while read -r event pod_name; do
    if [ "${event}" == "ADDED" ]; then
      collect_logs "${namespace}" "${pod_name}"
    elif [ "${event}" == "DELETED" ]; then
      list_log_file "${namespace}" "${pod_name}"
    fi
  done
done

















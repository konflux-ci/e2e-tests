#!/bin/bash

# use the set -e option to make the script exit immediately if a command fails.
set -e

# Array of default namespaces
default_namespaces=(
    "application-service"
    "build-service"
    "enterprise-contract-service"
    "gitops-service-argocd"
    "openshift-user-workload-monitoring"
    "integration-service"
    "internal-services"
    "jvm-build-service" 
    "openshift-service-ca"
    "openshift-service-ca-operator"
    "release-service")


main() {
    declare -g log_dir
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
    mkdir -p "${log_dir}" || { echo "Error: Could not create directory ${log_dir}"; exit 1; }

    # Read namespaces from a file if it exists, otherwise use the default namespaces array
    namespaces_file="namespaces.txt"
    if [ -f "${namespaces_file}" ]; then
      mapfile -t namespaces < "${namespaces_file}"
    else
      namespaces=("${default_namespaces[@]}")
    fi

    # Trap SIGINT (Ctrl+C) and call cleanup_and_exit function
    trap 'cleanup_and_exit' SIGINT

    # Start collecting new tailed log data from (namespaces in namespace array) namespace pods new containers
    collect_logs_from_existing_namespaces

    # Start collecting new data from all new tenant type namespace pods containers
    collect_logs_from_new_namespaces
}

# Function to tail logs of a created or existing pod containers in the background and store it in a distinct file
collect_logs() {
  local namespace=$1
  local pod_name=$2
  local container_name=$3
  local container_dir=$4
  local log_file="${container_dir}/${container_name}.log"

  echo "Collecting logs for container ${container_name} in pod ${pod_name} in namespace ${namespace} ..."
  kubectl logs -f "${pod_name}" -c "${container_name}"  -n "${namespace}" --tail=1 >> "${log_file}" 2>&1 &
}


# Function to list the log file of a deleted pod, send it to the remote store, and delete it if empty
list_log_file() {
  local namespace=$1
  local pod_name=$2
  local container_name=$3
  local log_file="${log_dir}/${namespace}/${pod_name}/${container_name}.log"

  # If the file exists then process it..
  if [ -e "${log_file}" ]; then
    # Check if the log file is empty and delete it if so
    if [ ! -s "${log_file}" ]; then
      echo "Log file ${log_file} is empty. Deleting..."
      rm "${log_file}"
    else
      # Send the log file to the remote store (not now..)
      echo "Logfile: ${log_file}"
    fi
  fi
}

# Function to clean up and exit the script
cleanup_and_exit() {
  echo "Received SIGINT. Cleaning up and exiting..."
  kill $(jobs -p) 2>/dev/null
  # rm -f "${log_dir}"/*.log
  exit 0
}

# Main loop to watch for created and deleted pods for current namespaces
collect_logs_from_existing_namespaces() {
  for namespace in "${namespaces[@]}"; do
    namespace_dir="$log_dir/$namespace"
    mkdir -p "$namespace_dir"
    # Watch for added or deleted pods in the new tenant namespace
    kubectl get pods --watch --namespace "$namespace" --output-watch-events --output jsonpath='{.type} {.object.metadata.name}{"\n"}' | while read -r event pod_name; do
      if [ "$event" == "ADDED" ]; then
        echo "New pod added: $pod_name in namespace $namespace"
        pod_dir="$namespace_dir/$pod_name"
        mkdir -p "$pod_dir"
        # Watch for added or deleted containers in the new pod
        kubectl get pods "$pod_name" --watch --namespace "$namespace" --output-watch-events --output jsonpath='{.type} {.object.spec.containers[*].name}{"\n"}' | while read -r event container_name; do
          if [ "$event" == "ADDED" ]; then
            echo "New container added: $container_name in pod $pod_name in tenant namespace $namespace"
            container_dir="$pod_dir"
            collect_logs "${namespace}" "${pod_name}" "${container_name}" "${container_dir}"
          elif [ "$event" == "DELETED" ]; then
            echo "Container deleted: $container_name in pod $pod_name in tenant namespace $namespace"
            list_log_file "${namespace}" "${pod_name}" "${container_name}"
          fi
        done &
      elif [ "$event" == "DELETED" ]; then
        echo "Pod deleted: $pod_name in tenant namespace $namespace"
      fi
    done &
  done
}


collect_logs_from_new_namespaces() {
  while true; do
    kubectl get namespaces --watch --output-watch-events | while read -r event; do
      event_type=$(echo "$event" | awk '{print $1}')
      namespace=$(echo "$event" | awk '{print $2}')

      if [ "$event_type" == "ADDED" ] && echo "$namespace" | grep -q ".*-tenant$"; then
      # Searching for tenant named namespaces
          echo "New tenant namespace added: $namespace"
          namespace_dir="$log_dir/$namespace"
          mkdir -p "$namespace_dir"

          # Watch for added or deleted pods in the new tenant namespace
          kubectl get pods --watch --namespace "$namespace" --output-watch-events --output jsonpath='{.type} {.object.metadata.name}{"\n"}' | while read -r event pod_name; do
            if [ "$event" == "ADDED" ]; then
              echo "New pod added: $pod_name in tenant namespace $namespace"
              pod_dir="$namespace_dir/$pod_name"
              mkdir -p "$pod_dir"

              # Watch for added or deleted containers in the new pod
              kubectl get pods "$pod_name" --watch --namespace "$namespace" --output-watch-events --output jsonpath='{.type} {.object.spec.containers[*].name}{"\n"}' | while read -r event container_name; do
                if [ "$event" == "ADDED" ]; then
                  echo "New container added: $container_name in pod $pod_name in tenant namespace $namespace"
                  container_dir="$pod_dir"
                  collect_logs "${namespace}" "${pod_name}" "${container_name}" "${container_dir}"
                elif [ "$event" == "DELETED" ]; then
                  echo "Container deleted: $container_name in pod $pod_name in tenant namespace $namespace"
                  list_log_file "${namespace}" "${pod_name}" "${container_name}"
                fi
              done &
            elif [ "$event" == "DELETED" ]; then
              echo "Pod deleted: $pod_name in tenant namespace $namespace"
            fi
        done &
      elif [ "$event_type" == "DELETED" ] && echo "$namespace" | grep -q ".*-tenant$"; then
        echo "New tenant namespace deleted: $namespace"
      fi
    done
  done
}

main "$@"; exit

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

    # Create a subdirectory to store named pipes
    declare -g event_pipe_subdir="event_pipes"
    # this subdirectory is used as event_pipe files to be used in the process_pod function

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

    # The event_pipe_subdir directory is used to store named pipes
    # Remove the event_pipe_subdir directory if it exists and create a new one
    if [ -d "$event_pipe_subdir" ]; then
      rm -rf "$event_pipe_subdir"
    fi
    mkdir -p "$event_pipe_subdir"


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


# Function to clean up and exit the script
cleanup_and_exit() {
  echo "Received SIGINT. Cleaning up and exiting..."
  kill $(jobs -p) 2>/dev/null
  # rm -rf "${log_dir}"/*.log
  rm -rf "$event_pipe_subdir"
  exit 0
}

# Main loop to watch for created and deleted pods for current namespaces
collect_logs_from_existing_namespaces() {
  for namespace in "${namespaces[@]}"; do
    process_namespace $namespace &
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
          process_namespace $namespace &
      elif [ "$event" == "DELETED" ]; then
          echo "tenant namespace deleted: $namespace"
      fi
    done
  done
}

# 
# ---new 
# 

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

# Function to process namespaces
process_namespace() {
  local namespace=$1
  namespace_dir="$log_dir/$namespace"
  mkdir -p "$namespace_dir"

  # Continuously watch the namespace's events for changes
  kubectl get pods -n "$namespace" --watch --output-watch-events --output jsonpath='{.type} {.object.metadata.name}{"\n"}' | while read -r event pod_name; do
    if [ "$event" == "ADDED" ]; then
      echo "New pod added: $pod_name in namespace $namespace"
      # Process the pod in parallel
      process_pod "$namespace" "$pod_name" &
    elif [ "$event" == "DELETED" ]; then
      echo "Pod deleted: $pod_name in namespace $namespace"
    fi
  done
}


# potential issue:
# Using wait to wait for all container events to be processed before reading the next set of events 
# can indeed cause you to miss fast-running containers with a short lifespan. 
# If a container starts and finishes between the wait and the next iteration of reading the container events, 
# it might not be detected.

# solution:
# To address this issue, you can use a background process to watch for events 
# and push them to a shared buffer, such as a named pipe (FIFO), 
# then read the events from that buffer in the main loop. 
# This way, you will not miss any events while waiting for the container event handling to complete.

# This version of the process_pod function creates a named pipe (FIFO) called 
# 'event_pipe' to store container events. It then starts a background process to watch for 
# events and pushes them to the named pipe. The main loop reads events from the named pipe 
# and processes them in parallel using the handle_container_event function.
# After processing all events, the loop waits for the event handlers to complete before reading the next set of events. 
# Finally, the named pipe is removed once the loop exits.

# We still need the wait command
# The wait command is used to ensure that all container events processed in parallel by the 
# handle_container_event function are completed before reading the next set of events 
# from the event_pipe. 

# Without the wait command, the main loop would continue to read and process new 
# events without waiting for the previous events to finish. This could lead to a situation where 
# multiple events are being handled simultaneously, and their processing might overlap or 
# interfere with each other. It could also cause excessive resource consumption if too many 
# events are being processed concurrently.

# By adding the wait command, we are ensuring that the script handles events in a 
# controlled manner, processing each set of events before moving on to the next one. 
# This approach reduces the chance of race conditions, ensures that resources are used efficiently, 
# and makes it easier to track and debug the script's behavior.

# This approach creates a unique named pipe for each process_pod function call 
# by incorporating the namespace and pod name into the event pipe name. 
# The cleanup process also removes the named pipe after the function is finished.

# Function to process pods
process_pod() {
  set -e

  local namespace=$1
  local pod_name=$2

  pod_dir="$namespace_dir/$pod_name"
  mkdir -p "$pod_dir"

  # Wait until the pod is in ContainersReady condition
  kubectl wait --timeout=120s --for=condition=ContainersReady pod/"$pod_name" -n "$namespace" 2>/dev/null

  declare -a processed_containers

  # Create a unique named pipe for each concurrent execution using a hash
  local event_pipe_hash=$(echo "${namespace}_${pod_name}" | md5sum | cut -f1 -d' ')
  # local event_pipe_subdir="event_pipes"
  local event_pipe="${event_pipe_subdir}/event_pipe_${event_pipe_hash}"
  
  # Create a named pipe to store container events
  mkfifo "$event_pipe"

  # Start a background process to watch for events and push them to the named pipe
  kubectl get pods "$pod_name" -n "$namespace" --watch --output-watch-events --output jsonpath='{.type} {.object.status.containerStatuses[*].name}{"\n"}' > "$event_pipe" &

  # while ! is_pod_in_terminal_state "$namespace" "$pod_name"; do
  while true; do
    if is_pod_in_terminal_state "$namespace" "$pod_name"; then
      break
    fi

    if read -r event container_names < "$event_pipe"; then
      IFS=' ' read -ra container_array <<< "$container_names"

      for container_name in "${container_array[@]}"; do
        # Handle the container event in parallel
        handle_container_event "$namespace" "$pod_name" "$pod_dir" "$event" "$container_name" "${processed_containers[*]}" &

        # Mark the container as processed
        processed_containers+=("$container_name")
      done
      wait # Wait for all container events to be processed before reading the next set of events
    fi

    sleep 1 # Reduce the sleep interval to check pod state more frequently
  done

  # Clean up the named pipe
  rm "$event_pipe"
}



# Function to check if the pod is in a terminal state
is_pod_in_terminal_state() {
  local namespace=$1
  local pod_name=$2
  local pod_phase

  pod_phase=$(kubectl get pod "$pod_name" -n "$namespace" -o jsonpath='{.status.phase}')
  [[ "$pod_phase" == "Succeeded" ]] || [[ "$pod_phase" == "Failed" ]]
}


# Function to handle the container events
handle_container_event() {
  local namespace=$1
  local pod_name=$2
  local pod_dir=$3
  local event=$4
  local container_name=$5
  local processed_containers=$6

  if [ "$event" == "ADDED" ]; then
    echo "New container added: $container_name in pod $pod_name in tenant namespace $namespace"
    container_dir="$pod_dir"

    if ! is_processed "${processed_containers[*]}" "$container_name"; then
      echo "Processing container: $container_name in pod $pod_name in namespace $namespace"
      # Process the container in parallel
      process_container "$namespace" "$pod_name" "$container_name" "$container_dir" &
      # Mark the container as processed
      processed_containers+=("$container_name")
    fi
  elif [ "$event" == "DELETED" ]; then
    echo "Container deleted: $container_name in pod $pod_name in namespace $namespace"
  fi
}

# Function to process containers within a pod
process_container() {
  local namespace=$1
  local pod_name=$2
  local container_name=$3
  local container_dir=$4

  local container_name=$3
  # Wait until the container is in ready state or until timeout is reached
  # Define timeout value
  timeout=60
  # Start timer
  start=$(date +%s)
  # Loop until the container is ready and in Running state
  while true; do
    # Check the container's state and readiness.
    container_status=$(kubectl get pods "$pod_name" -n "$namespace" -o jsonpath="{.status.containerStatuses[?(@.name=='$container_name')].state}" | sed -n 's/{"\([^"]*\)".*/\1/p')
    container_ready=$(kubectl get pods "$pod_name" -n "$namespace" -o jsonpath="{.status.containerStatuses[?(@.name=='$container_name')].ready}")

    # If the container is ready and in Running state, break the loop
    if [ "$container_ready" == "true" ] && [ "$container_status" == "running" ]; then
      echo "Container $container_name in pod $pod_name in namespace $namespace is in ready and in running state"
      break
    fi

    # Check if timeout has been reached
    now=$(date +%s)
    if [ $((now - start)) -ge $timeout ]; then
      echo "Timeout of $timeout seconds reached while waiting for the container $container_name in pod $pod_name in namespace $namespace to be in Running state and ready"
      # Reset the start time to the current time
      start=$(date +%s)
    fi

    # Sleep for a short duration before checking again
    sleep 5
  done

  # Collect logs from the container
  collect_logs "${namespace}" "${pod_name}" "${container_name}" "$container_dir" 
}


# Function to check if a container is in the list of processed containers
is_processed() {
  local processed_containers_ref=$1
  shift
  local container=$1

  for processed_container in "${processed_containers_ref[@]}"; do
    if [[ "$container" == "$processed_container" ]]; then
      return 0
    fi
  done
  return 1
}

# Function to get the list of containers in a pod
get_container_list() {
  local namespace=$1
  local pod_name=$2

  kubectl get pods "$pod_name" -n "$namespace" -o jsonpath='{.spec.containers[*].name}'
}

#
# --end
#

main "$@"; exit

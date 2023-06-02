#!/bin/bash

set +e

 # Log directory configuration
 # Format:  ./snapshot-collect-pod-logs.sh logs-${USER}-$(date +%Y-%m-%d)  

 # if no parameter given, a default will be used for the log_dir
 if [ -z "$1" ]; then
   # Set default log directory
   timestamp=$(date +%Y-%m-%d-%H-%M-%S)
   log_dir="logs-${USER}-$timestamp"
 else
   # Use log directory provided as script argument
   log_dir="$1"
 fi

interval=$(( 5 * 60 ))  # 5 minutes

# Flag tosignal if a signal has been received 
signal_received=0

# Function to handle signals
function handle_signal() {
    echo "Script terminated by user"
    signal_received=1
}

# Trap signals
trap 'handle_signal' SIGINT SIGTERM

while true; do
    # check if a signal has been received
    if [[ signal_received -eq 1 ]]; then
      echo "Exiting safely"
      exit 0
    fi

    echo "Wait for the interval in seconds before processing the logs gathering loop.."
    sleep $interval

    # check if there are any namespaces available
    namespaces=$(kubectl get ns -o jsonpath='{.items[*].metadata.name}')
    if [[ -z $namespaces ]]; then
      echo "No namespaces available, Skipping this iteration"
      echo "Wait for the interval in seconds before processing the logs gathering loop.."
      sleep $interval

      continue
    fi

    # Iterate through namespaces
    echo "$namespaces" | tr ' ' '\n' | while read -r namespace; do
        echo "Processing namespace: $namespace"

        # Create namespace directory if it doesn't exist
        mkdir -p "$log_dir/$namespace"

        # get pods in namespace
        pods=$(kubectl get pods -n $namespace -o jsonpath='{.items[*].metadata.name}')
        if [[ -z $pods ]]; then
          echo "No pods in namespace: $namespace, iterate with next namespace if exists"
          continue
        fi

        # Iterate through pods
        echo "$pods" | tr ' ' '\n' | while read -r pod; do
            echo "  Processing pod: $pod"

            # Create pod directory if it doesn't exist
            mkdir -p "$log_dir/$namespace/$pod"

            # get containers in the pod
            containers=$(kubectl get pods -n $namespace $pod -o jsonpath='{.spec.containers[*].name}')
            if [[ -z $containers ]]; then
              echo "No containers exist in pod: $namespace/$pod, iterate with next pod if exists"
              continue
            fi

            # Iterate through containers
            for container in $containers; do
                echo "    Processing container: $container"
                container_dir="$log_dir/$namespace/$pod/$container"

                # Create container directory if it doesn't exist
                mkdir -p "$container_dir"
                
                # Generate a new timestamp
                timestamp=$(date +%Y%m%d%H%M%S)

                # Fetch container logs
                log=$(kubectl logs -n $namespace $pod $container)
                
                # If log is not empty, delete existing logs and write the new log to a file
                if [[ ! -z "$log" ]]; then
                    rm -f "$container_dir"/*.log
                    echo "$log" > "$container_dir/$container-$timestamp.log"
                fi
            done
        done
    done
done

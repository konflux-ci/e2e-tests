#!/bin/bash

# Initialize sums
sum_rtt=0
sum_skew=0

for i in {1..3}; do
    # Step 1: Measure RTT in nanoseconds and calculate in seconds with fractional part
    start=$(date +%s%N) # Start time in nanoseconds
    oc exec prometheus-k8s-0 -n openshift-monitoring -- date -u +"%Y-%m-%dT%H:%M:%S%z" > /dev/null
    end=$(date +%s%N)   # End time in nanoseconds

    # Calculate round-trip time in seconds with fractions
    rtt=$(echo "scale=3; ($end - $start) / 1000000000" | bc) # Round to three decimal places
    sum_rtt=$(echo "$sum_rtt + $rtt" | bc)

    # Step 2: Get remote time and local time, one after the other
    remote_time=$(oc exec prometheus-k8s-0 -n openshift-monitoring -- date -u +"%Y-%m-%dT%H:%M:%S.%N%z")
    local_time=$(date -u +"%Y-%m-%dT%H:%M:%S.%N%z")

    remote_time_epoch=$(date -ud "$remote_time" +"%s.%3N") # Truncate to milliseconds
    local_time_epoch=$(date -ud "$local_time" +"%s.%3N") # Truncate to milliseconds

    # Adjust for half of the round-trip time
    latency_correction=$(echo "scale=3; $rtt / 2" | bc) # Round to three decimal places
    remote_time_corrected_epoch=$(echo "scale=3; $remote_time_epoch - $latency_correction" | bc) # Round to three decimal places


    # Step 3: Calculate skew with fractional part
    time_skew=$(echo "scale=3; $local_time_epoch - $remote_time_corrected_epoch" | bc) # Round to three decimal places
    sum_skew=$(echo "$sum_skew + $time_skew" | bc)

    # Optional: Output each measurement
    # echo "Measurement $i: RTT = $rtt seconds, Time Skew = $time_skew seconds"
done

# Calculate averages
avg_rtt=$(echo "scale=3; $sum_rtt / 3" | bc)
avg_skew=$(echo "scale=3; $sum_skew / 3" | bc)

# Output the averages
echo "Average RTT: $avg_rtt seconds"
echo "Average Time Skew: $avg_skew seconds"

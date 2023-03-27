# JIRA task - https://issues.redhat.com/browse/STONE-831 
# The bash script is collect-openshift-pod-logs.sh


The script logic:
================
Log directory configuration
Format:  ./collect-openshift-pod-logs.sh logs-${USER}-$(date +%Y-%m-%d)  
if no parameter given, a default will be used for the log_dir - logs-${USER}-$(date +%Y-%m-%d)

1. The script will run in the background process
2. Per each namespace, it will tail logs of a created or existing pod in the background and store it in a distinct filename
3. All log files will be stored under a main log_dir of type log_dir="logs-${USER}-$(date +%Y-%m-%d)"
4. If the script runs in the forground, the user may stop the script by sending a Trap SIGINT (Ctrl+C) which will call cleanup_and_exit function and exit safely 
   the script's processes and also deleting all the collected log files
5. As for the namespaces - It will read namespaces from a namespace.txt file if it exists, otherwise use the default namespaces array
6. We may choose any namespaces wanting in the default_namespaces and/or in the namespaces.txt file
7. The default namespaces within the script which I used relates to all the services namespaces - oc get ns | grep -i service | cut -d' ' -f1 
8. The full namespaces.txt file will get also the following namespaces:
    a. oc get ns | grep -i service | cut -d' ' -f1 > namespaces.txt
    b. oc get ns | grep -i toolchain | cut -d' ' -f1 >> namespaces.txt
    c. echo "tekton-results" >> namespaces.txt 


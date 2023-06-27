## JIRA Task
[RHTAP-869](https://issues.redhat.com/browse/RHTAP-869)
## Bash Script Name
The bash script is `snapshot-collect-pod-logs.sh`



## Log directory configuration
Format: `./snapshot-collect-pod-logs.sh logs-${USER}-$(date +%Y-%m-%d-%H-%M-%S)`
If no parameter is given, a default log_dir will be used: `log_dir="logs-${USER}-$(date +%Y-%m-%d-%H-%M-%S)"`
## Script Logic
1. The script will run in a continuous loop iterating through all namespaces/pods/containers.
2. All log files will be stored under a main log_dir of type `log_dir`.
3. For each container log, if not empty:
   - Existing container log will be removed.
   - The new log will be stored in the proper hierarchy.

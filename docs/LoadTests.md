# Konflux Load Test

## Result collection and monitoring (stage/probe)

When load-test results are collected (e.g. by `collect-results-probe.sh`), the following happens:

1. **POD, task and step parsing**  
   A shell pipeline (find + jq) scans `ARTIFACT_DIR/collected-data/` for `collected-taskrun-*.json` files. From each TaskRun JSON it reads `.metadata.namespace`, `.status.podName`, `.metadata.name` (task name), and `.status.steps[].name`, then aggregates into:
   - `ARTIFACT_DIR/get-pod-step-names.json` – machine-readable list of `{ "pods": [ { "namespace", "pod_id", "task_name", "steps": [...] }, ... ] }`

2. **Monitoring config expansion**  
   `ci-scripts/utility_scripts/append-pod-step-monitoring.py` reads the repo’s `cluster_read_config.yaml` and `get-pod-step-names.json`, then appends one `monitor_task_step(namespace, pod_id, task_name, step_name, ...)` Jinja call per (task, step). Metrics are stored under **task name** (stable across runs) as `measurements.tasks[<task_name>].step[<step_name>].memory` and `.cpu`. The Prometheus query uses container name `step-<step_name>` to match Tekton step containers.

3. **Artifacts produced**  
   Under each run’s `ARTIFACT_DIR`: `get-pod-step-names.json`, `cluster_read_config.yaml_modified`.

## Running the script
1. Change your directory to `tests/load-tests`
2. Environment variables are required to set for the e2e framework that the load test uses. Refer to [Installation](Installation.md).
3. Run the bash script
```
./run.sh
```

For help run `go run loadtest.go --help`.

## How does this work
1. Start a thread for each user journey (here, Concurrency = 1)
  1. Start a thread for each per-application journey (here, ApplicationsCount = 4)
    1. Create Application
    2. Create IntegrationTestScenario
    3. Create ComponentDetectionQuery
    4. Start a thread for each per-component journey (here, ComponentsCount = 1)
      1. Create Component with annotation “skip-initial-checks” set to true/false (here, PipelineSkipInitialChecks = true/false)
      2. Wait for build PipelineRun to finish
      3. Wait for test PipelineRun to finish

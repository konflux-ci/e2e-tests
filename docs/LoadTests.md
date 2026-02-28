# Konflux Load Test

## Result collection and monitoring (stage/probe)

When load-test results are collected (e.g. by `collect-results-probe.sh`), the following happens:

1. **POD and step parsing**  
   Script `ci-scripts/utility_scripts/get-pod-step-names.py` scans `ARTIFACT_DIR/collected-data/<namespace>/<run_id>/` for log files matching `pod-<POD_ID>-pod-step-<STEP_NAME>.log`. It extracts POD IDs (Tekton Task pods) and step names (containers) and writes:
   - `ARTIFACT_DIR/pod-step-names.json` – machine-readable list of `{ "namespace", "pod_id", "steps": [...] }`
   - `ARTIFACT_DIR/pod-step-names.log` – human-readable dump of the same

2. **Monitoring config backup and expansion**  
   - The repo’s `ci-scripts/stage/cluster_read_config.yaml` is copied to `ARTIFACT_DIR/cluster_read_config.yaml_orig`.
   - `ci-scripts/utility_scripts/append-pod-step-monitoring.py` reads `pod-step-names.json` and appends one `monitor_pod_container(namespace, pod_id, step, ...)` Jinja call per (POD, step) to that config.
   - The result is written to `ARTIFACT_DIR/cluster_read_config.yaml_modified`, which is then used for Prometheus monitoring collection (per-pod, per-container CPU and working-set memory).

3. **Artifacts produced**  
   Under each run’s `ARTIFACT_DIR`: `cluster_read_config.yaml_orig`, `cluster_read_config.yaml_modified`, `pod-step-names.json`, `pod-step-names.log`.

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

# Konflux Load Test

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

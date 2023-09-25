# Integration Service

The Integration Service manages the lifecycle, deployment and integration testing of applications and components. It handles snapshots, environments, pipelines and deployments, ensuring applications run efficiently and are deployed correctly.


## Happy Path Tests

Happy path testing describes tests that focus on the most common scenarios while assuming there are no exceptions or errors.
### Basic E2E Tests (within `integration.go`):

- Testing for successful creation of applications and components.
- Asserting the successful creation of a snapshot after a push event.
- Checking if the BuildPipelineRun is successfully triggered and completed.
- Asserting the signing of BuildPipelineRun.
- Validating the successful creation of a SnapshotEnvironmentBinding.
- Validating the successful creation of a Release.
- Validating the Global Candidate is updated
- Verifying that all the Integration PipelineRuns finished successfully.
- Checking that a snapshot gets successfully marked as 'passed' after all Integration pipeline runs finished.

### E2E tests of Namespace-backed Environments (within `namespace-backed-environments.go`):

- Testing for successful creation of applications, components, DeploymentTargetClass, Environment, and IntegrationTestScenario
- Checking if the BuildPipelineRun is successfully triggered and completed.
- Asserting the signing of BuildPipelineRun.
- Validating the successful creation of a Snapshot, an Ephemeral environment, and an Integration PipelineRun.
- Verifying that the Integration PipelineRun succeeded.
- Asserting that the Snapshot was marked as Passed.
- Verifying that the Epehemeral environment and related SnapshotEnvironmentBinding got deleted.


## Negative Test Cases

### Failed IntegrationTestScenarios (within `integration.go`):

- Creating an IntegrationTestScenario that is expected to fail.
- Asserting that a snapshot is marked as failed.
- Validating that no Release CRs and no SnapshotEnvironmentBinding are created in certain scenarios.
- Checking that the global candidate does not get updated unexpectedly.


### Failed Ephemeral Environment provisioning due to missing DeploymentTargetClass (within `namespace-backed-environments.go`):
- Verifying that entities like deploymentTargetClass and GitOpsCR don't exist under certain conditions.
- Asserting that no GitOpsCR is created for a non-existing deploymentTargetClass.
- Checking that a snapshot doesn't get marked as 'passed' under specific conditions.
  

## Running e2e tests

- [Prepare the cluster.](https://github.com/redhat-appstudio/e2e-tests#install-appstudio-in-e2e-mode)


- To run service-level e2e test suite: integration.go

```bash
$ ./bin/e2e-appstudio --ginkgo.focus="integration-service-suite" –ginkgo.vv
```








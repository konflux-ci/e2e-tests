# Integration Service

The Integration Service manages the lifecycle, deployment, and integration testing of applications and components. It handles snapshots, environments, pipelines, and deployments, ensuring applications run efficiently and are deployed correctly.

## Test Suites

### 1. Basic E2E Tests within `integration.go`
These tests cover the end-to-end integration service workflow, verifying the successful execution of key processes such as component creation, pipeline runs, snapshot generation, and release management under ideal conditions.

### 2. E2E Tests within `status-reporting-to-pullrequest.go`
This suite tests the status reporting of integration tests to GitHub Pull Requests (PRs). It ensures that integration test outcomes are reflected accurately in the PR's CheckRuns, including both successful and failed tests.

### 3. E2E Tests within `gitlab-integration-reporting.go`
This suite verifies the reporting of integration test statuses to GitLab Merge Requests (MRs). It ensures the proper status updates for both successful and failed tests are reflected in the MR's CommitStatus.

### 4. E2E Tests within `integration-with-env.go`
This suite tests the integration service's interaction with ephemeral environments, ensuring correct handling of pipelines, snapshots, and environment cleanup.

---

## Happy Path Tests

Happy path testing describes tests that focus on the most common scenarios while assuming there are no exceptions or errors.

### 1. Happy Path Tests within `integration.go`
Checkpoints:
- Testing for successful creation of applications and components.
- Checking if the BuildPipelineRun is successfully triggered, contains the finalizer, and was completed.
- Asserting the signing of BuildPipelineRun.
- Validating the successful creation of a Snapshot, and removal of finalizer from BuildPipelineRun.
- Verifying that all Integration PipelineRuns finished successfully.
- Checking that the status of integration tests was reported to the Snapshot.
- Checking that a snapshot gets successfully marked as 'passed' or 'failed' after all Integration PipelineRuns finished.
- Handling of Skipped Integration Tests.
- Creating a related PR after Build PipelineRun completion for components with custom branches.
- Creating a new IntegrationTestScenario that is expected to pass.
- Updating the Snapshot with a re-run label for the new scenario.
- Validating that a new Integration PLR is created and finished.
- Asserting that the Snapshot doesn't contain the re-run label, and contains the name of the re-triggered PipelineRun.
- Validating that a new Snapshot is created after push events.
- Validating the Global Candidate is updated.
- Validating the successful creation of a Release.
- Checking for the creation of a Release Plan.
- Asserting the successful creation of a snapshot after a push event.
- Management of Pull Requests and Branches.
- Re-running Integration Tests.
- Finalizer Removal from Integration PipelineRuns.

### 2. Happy Path Tests within `status-reporting-to-pullrequest.go`
Checkpoints:
- Creating two IntegrationTestScenarios: one that should pass.
- Creating a Pull Request (PR) from a custom branch.
- Triggering a Build PipelineRun and validating it is completed successfully.
- Asserting the creation of a PaC (Pipelines as Code) init PR in the component repository.
- Asserting the correct status reporting for the Build PipelineRun in the PR's CheckRun.
- Verifying that the successful Integration PipelineRun is reported correctly in the CheckRun.

Push Event Tests:
- Creating a test commit on the main branch.
- Verifying build pipeline triggers and completes for the push event.
- Checking integration test results are reported to the commit's status checks:
  * Success status for passing test scenario
  * Failure status for failing test scenario
- Ensuring proper cleanup of test artifacts.
### 3. Happy Path Tests within `gitlab-integration-reporting.go`
Checkpoints:
- Creating two IntegrationTestScenarios: one expected to pass.
- Creating a custom branch that triggers a Merge Request (MR).
- Triggering a Build PipelineRun and ensuring it completes successfully.
- Verifying that the Build PipelineRun is reflected correctly in the MR's CommitStatus.
- Ensuring the successful Integration PipelineRun is reported as "Pass" in the MR's CommitStatus.
- Ensuring that the MR notes show the successful status of the integration test.
- Merge MR and repeat three tests above.

### 4. Happy Path Tests within `integration-with-env.go`
Checkpoints:
- Creating an IntegrationTestScenario pointing to an integration pipeline with environment settings.
- Successfully creating applications and components.
- Verifying that BuildPipelineRuns are triggered, contain finalizers, and finish successfully.
- Asserting the signing of the BuildPipelineRun and successful Snapshot creation.
- Ensuring that the Integration PipelineRuns finish successfully and report status to the Snapshot.
- Verifying that snapshots are marked as 'passed' after Integration PipelineRuns finish.
- Ensuring that CronJobs such as "spacerequest-cleaner" exist.
- Handling space requests in the namespace and ensuring they are created correctly.
- Verifying proper cleanup after successful deletion of the Integration PipelineRun.

---

## Negative Tests

Negative testing focuses on how the system behaves under invalid, unexpected, or failing conditions to ensure robustness and error handling.

### 1. Negative Test Cases within `integration.go`
Checkpoints:
- Creating an IntegrationTestScenario that is expected to fail.
- Asserting that a snapshot is marked as failed.
- Handling failure scenarios where re-run integration test scenarios are created and ensuring failures are reported correctly.
- Asserting that a snapshot is still marked as failed.
- Validating that no Release CRs are created in certain scenarios.
- Checking that the global candidate does not get updated unexpectedly.
- Ensuring proper status reporting for failed Integration PipelineRuns and snapshots in PRs (or GitLab MR's CommitStatus).

### 2. Negative Test Cases within `status-reporting-to-pullrequest.go`
Checkpoints:
Checkpoints:
- Creating two IntegrationTestScenarios: one that should fail.
- Verifying that failed Integration PipelineRuns are reported correctly in the PR's CheckRun.
- Checking that snapshots are marked as 'failed' if any test fails.
- For push events:
  * Verifying failed tests are properly reported in commit status checks
  * Ensuring snapshots are marked as failed for failing tests
  * Validating error handling for invalid commits or failed builds

### 3. Negative Test Cases within `gitlab-integration-reporting.go`
Checkpoints:
- Creating two IntegrationTestScenarios: one expected to fail.
- Verifying that the failing Integration PipelineRun is reflected as "Fail" in the MR's CommitStatus.
- Ensuring that MR notes show the failure status of the integration test.
- Asserting that no releases are triggered if any integration test fails.
- Merge MR and repeat three tests above.

### 4. Negative Test Cases within `integration-with-env.go`
Checkpoints:
- Verifying that integration pipelines are marked as failed when tests do not pass.
- Checking that snapshots are marked as 'failed' when Integration PipelineRuns do not finish successfully.
- Ensuring that space requests are deleted correctly in failed scenarios.

---

## Running E2E Tests

- [Prepare the cluster.](https://github.com/redhat-appstudio/e2e-tests#install-appstudio-in-e2e-mode)

- To run service-level E2E test suite: `integration.go`

```
 ./bin/e2e-appstudio --ginkgo.focus="integration-service-suite" –ginkgo.vv
```

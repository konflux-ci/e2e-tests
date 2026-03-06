# Release Service e2e tests suite

Contains E2E tests related to [Release Service](https://github.com/konflux-ci/release-service).

The test cases in this suite are to test release-service functionalities.
All tests must have the label `release-service`. 

## The happy path test (happy_path.go)

This test verifies the functionality of the release-service in watching for new release CRs then performing the necessary actions based on the design. 
The Release CR is created by creating Application and component CRs to trigger build Pipeline. Once the build PipelineRun succeeds, Snapshot and Release CRs are created by integration-service if an auto ReleasePlan is configured. Then the Release CR triggers release pipelineRun and the Status of Release CR will be updated accordingly after the release pipelineRun is finished.

Checkpoints:
  - A build PipelineRun is created in the dev namespace.
  - The build PipelineRun passes.
  - The Release CR is created.
  - The release PipelineRun is successfully created in the managed namespace.
  - The PipelineRun should pass.
  - The Release passes.
  - Validate that the Release object is referenced by the PipelineRun.

## The happy path with deployment (happy_path_with_deployment.go)

This test is designed to test release-service functionalities with an environment defined. Once the release successfully passes, the application and components will be copied to the specified environment.

Checkpoints:
  - A build PipelineRun is created in the dev namespace.
  - The build PipelineRun passes.
  - The Release CR is created.
  - The release PipelineRun is successfully created in the managed namespace.
  - The release PipelineRun is expected to pass.
  - The Release passes.
  - Copy the application and component to the environment and ensure the process succeeds.

## Negative e2e-tests (missing_release_plan_and_admission.go)

This test file includes two negative test cases.

1. A release CR will fail if ReleasePlanAdmission is missing.

   Checkpoints:
     - Ensure that Release CR fails on Validation and on Release, with a proper message printed out to the user. 

2. A release CR will fail if ReleasePlan is missing .

   Checkpoints:
     - Ensure that Release CR fails on Validation and on Release, with a proper message printed out to the user.

## The happy path with pushing to external registry on self-hosted Quay (push_to_external_registry_self_hosted_quay.go)

This test is designed to run the release pipeline `push-to-external-registry` against a self-hosted Quay instance.

Prerequisites:
  - A self-hosted Quay instance must be running in the cluster.
  - The `init-quay` task must have run, creating:
    - A `quay-test-config` ConfigMap in the Quay namespace (with `quay-internal-host`, `image-digest`, and `dest-repo`).
    - A `quay-robot-credentials` Secret in the Quay namespace.
    - A `quay-admin-token` Secret in the Quay namespace.

Checkpoints:
  - A Release CR is created in the dev namespace.
  - The release PipelineRun succeeds in the managed namespace.
  - The Release is marked as succeeded.

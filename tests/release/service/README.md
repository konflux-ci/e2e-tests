# Release Service e2e tests suite

Contains E2E tests related to [Release Service](https://github.com/redhat-appstudio/release-service).

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

## Ensure ReleasePlan has owner references set (release_plan_owner_ref.go)

This test is designed to ensure that the ReleasePlan controller reconciles ReleasePlans to have an owner reference for its application.

Checkpoints:
  - The ReleasePlan has an owner reference for the Application.
  - If the Application is deleted, the ReleasePlan is also deleted.

## Negative e2e-tests (missing_release_plan_and_admission.go)

This test file includes two negative test cases.

1. A release CR will fail if ReleasePlanAdmission is missing.

   Checkpoints:
     - Ensure that Release CR fails on Validation and on Release, with a proper message printed out to the user. 

2. A release CR will fail if ReleasePlan is missing .

   Checkpoints:
     - Ensure that Release CR fails on Validation and on Release, with a proper message printed out to the user.

# Release e2e tests

Contains E2E tests related to [Release Service](https://github.com/redhat-appstudio/infra-deployments/tree/main/components/release).

This document will detail the e2e tests included for the release suite.

## e2e tests
### 1. The manual good path (release.go)
   This test verifies the functionality of the release-service in watching for new release CRs then performing the necessary actions based on the design.

      Steps:
       - Create the following CRs: Snapshot, ReleaseStrategy, ReleasePlan, EnterpriseContractPolicy, ReleasePlanAdmission, and Release.

      Expected Results:
      - The release PipelineRun is successfully created in the managed namespace.
      - The PipelineRun should pass.
      - The Release passes.
      - Validate that the Release object is referenced by the PipelineRun.

### 2. Automated good path (e2e-test-default-bundle.go)
   Test focuses on running the release pipeline when a user creates all the required CRs. Once the component is created, we expect the build pipeline to pass. Integration-service will create a release CR thus release-service will trigger release pipeline `release`, we expect it pass and release CR to be successful.

      Steps:
       - Create the following CRs: PVC, ServiceAccount, ReleaseStrategy, ReleasePlan, EnterpriseContractPolicy, ReleasePlanAdmission, Application, and Component.

      Expected Results:
       - A build PipelineRun is created in the dev namespace.
       - The build PipelineRun passes.
       - The release PipelineRun is successfully created in the managed namespace.
       - The release PipelineRun is expected to pass.
       - The Release passes.

### 3. Automated good path with deployment (e2e-test-default-with-deployment.go)

   This test is designed to run the release pipeline pipeline-deploy-release and involves defining the environment. Once the release successfully passes, the application and components will be copied to the specified environment.

      Steps:
       - Create the following CRs: PVC, ServiceAccount, ReleaseStrategy, ReleasePlan, EnterpriseContractPolicy, ReleasePlanAdmission, Environment, Application, and Component.            
      Expected Results:
       - A build PipelineRun is created in the dev namespace.
       - The build PipelineRun passes.
       - The release PipelineRun is successfully created in the managed namespace.
       - The release PipelineRun is expected to pass.
       - The Release passes.
       - Copy the application and component to the environment and ensure the process succeeds.

### 4. Automated good path with pushing to Pyxis stage (e2e-test-push-image-to-pyxis.go)

   This test is designed to run the release pipeline `pipeline-push-to-external-registry` and validates that the artifacts are successfully pushed to Pyxis.

      Steps:
       - Create the following CRs: PVC, ServiceAccount, ReleaseStrategy, ReleasePlan, EnterpriseContractPolicy, ReleasePlanAdmission, Application, and two Components.            
      Expected Results:
       - A build PipelineRun is created in the dev namespace.
       - The build PipelineRun passes.
       - The release PipelineRun is successfully created in the managed namespace.
       - The release PipelineRun is expected to pass.
       - The Release passes.
       - Validate that the SBOM for both components were uploaded to Pyxis stage.


### 5. Negative e2e-tests
We use manual tests to cover negative tests, using filtering labels in Ginkgo framework to enable users to run negative e2e-tests whether all together or each one separately:

**Run all negative release e2e-tests**
```bash
./bin/e2e-appstudio --ginkgo.junit-report=report.xml  --ginkgo.focus="release" --ginkgo.label-filter="release-neg" --ginkgo.vv
```
**Run one specific negative e2e-test**
```bash
e2e-test = negMissingReleaseStrategy || negMissingReleasePlan || negMissingReleasePlanAdmission
./bin/e2e-appstudio --ginkgo.junit-report=report.xml  --ginkgo.focus="release" --ginkgo.label-filter="<e2e-test>" --ginkgo.vv
```
   #### 5.1. A release CR will fail if ReleasePlanAdmission is missing (neg-missing-releaseplanadmission.go).

      Steps:
       - Create the following CRs: Snapshot, ReleaseStrategy, ReleasePlan, EnterpriseContractPolicy and Release          
      Expected Results:
       - We ensure that Release CR fails on Validation and on Release, with a proper message printed out to the user. 

   #### 5.2. A release CR will fail if ReleasePlan is missing (neg-missing-releaseplan.go).

      Steps:
       - Create the following CRs: Snapshot, ReleaseStrategy, ReleasePlanAdmission, EnterpriseContractPolicy and Release          
      Expected Results:
       - We ensure that Release CR fails on Validation and on Release, with a proper message printed out to the user.

   #### 5.3. A release CR will fail if ReleaseStrategy is missing (neg-missing-releasestrategy.go).

      Steps:
       - Create the following CRs: Snapshot, ReleasePlan, ReleasePlanAdmission, EnterpriseContractPolicy and Release          
      Expected Results:
       - We ensure that Release CR fails on Validation and on Release, with a proper message printed out to the user.

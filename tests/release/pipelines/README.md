# Release pipelines e2e tests suite

This suite contains e2e-tests for testing release pipelines from repository [release-service-catalog](https://github.com/redhat-appstudio/release-service-catalog/tree/development).
All tests must have the label `release-pipelines`.

## Branch alignment
 * Below are the branches in [release-service-catalog](https://github.com/redhat-appstudio/release-service-catalog/tree/development). All these branches will be tested per requirement.  
   - production	: this branch will be used for RHTAP production environment
   - stage	: this branch will be used for RHTAP stage environment
   - development: this branch is the default branch for development

## Test cases 
### The happy path with pushing to Pyxis stage (rh_push_to_external_registry.go)

This test is designed to run the release pipeline `rh-push-to-external-registry` and validates that the artifacts are successfully pushed to Pyxis.

Checkpoints:
  - Two build PipelineRuns are created in the dev namespace.
  - The build PipelineRuns pass.
  - The release PipelineRuns are successfully created in the managed namespace.
  - The release PipelineRuns are expected to pass.
  - The Releases pass.
  - Validate that the SBOM for both components were uploaded to Pyxis stage.

### The happy path with pushing to external registry (push_to_external_registry.go)

This test is designed to run the release pipeline `push-to-external-registry`. 

Checkpoints:
  - A build PipelineRun is created in the dev namespace.
  - The build PipelineRun passes.
  - The release PipelineRun is successfully created in the managed namespace.
  - The release PipelineRun is expected to pass.
  - Test if the image from the snapshot is pushed to Quay
  - The Release passes.

### The happy path of FBC related tests (fbc_release.go)

This test is designed to run the release pipeline `fbc_release` related test and validates that the artifacts are successfully pushed. There are two test cases currently in this file. One of them is the happy path of releasing FBC related fragments and the other is for testing FBC hotfix releasing.

Prerequisites: 
   - Export the following environment variables:
       - TOOLCHAIN_API_URL_ENV	: Offline token used for getting Keycloak token in order to authenticate against stage/prod cluster
       - KEYLOAK_URL_ENV	: Keycloak URL used for authentication against stage/prod cluster
       - OFFLINE_TOKEN_ENV 	: Toolchain API URL used for authentication against stage/prod cluster
   - The tests will run on two dedicated namespaces, so a user not part of them need to request access to the following namespaces:
       - dev-release-team-tenant
       - managed-release-team-tenant

Checkpoints:
  - A build PipelineRun is created in the dev namespace.
  - The build PipelineRun passes.
  - The Release CR is created.
  - The release PipelineRun is successfully created in the managed namespace.
  - The release PipelineRun is expected to pass.
  - The Release passes.

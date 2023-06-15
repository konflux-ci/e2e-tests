# MVP demo test

### Prerequisites for running this test against your own cluster
1. Fork https://github.com/redhat-appstudio-qe/hacbs-test-project and https://github.com/redhat-appstudio-qe/strategy-configs to your GitHub org (specified in `MY_GITHUB_ORG` env var)
2. Make sure the cluster you are about to run this test against is public (i.e. hosted on a public cloud provider)

### Description
This test simulates the "advanced -> build -> scan -> test -> release" user journey

### Test steps
1. Setup
   1. Create a user and a related (dev) namespace, create a managed namespace used by release service for validating and releasing the built image
   2. Create required resources in managed-namespace
      1. Secret with container image registry credentials (used for pushing the image to container registry)
      2. Service account that mounts the secret, including required roles and rolebindings
      3. PVC for the release pipeline
      4. Secret with cosign-public-key (used for validating the built image by EC)
      5. Release plan for the targeted application and (user) namespace
      6. Release strategy, Release plan admission
      7. Enterprise contract policy
   3. Create a new branch in a java project on GitHub (that is used as a git source of a component) that will be used as a base branch for PaC pull request (so we don't pollute default branch - `main`)
   4. Create a new application, test environment
   5. Create a new component that will trigger the "advanced" build pipelinerun
   6. Create JBSConfig and related secret in user's (dev) namespace that will trigger jvm-build-service to deploy a jvm-cache to the dev namespace, which will be used for caching java dependencies during build

2. Test scenario
   1. Verify that the initial PaC pull request was created in the component's repo (this will also trigger an advanced build pipelinerun)
   2. After merging the PR, there should be another build pipelinerun triggered in user namespace
   3. Make sure the pipelinerun completes successfully
   4. Make sure that the resulting SBOM file can be pulled from the container registry (where also the image was pushed) and it is saved in expected format
   5. Make sure that integration test pipelinerun is created and completes successfully
   6. The release pipeline should succeed and the release should be marked as successful
   7. Make sure JVM build service is used for rebuilding java component's dependencies and that all related dependency builds complete successfully
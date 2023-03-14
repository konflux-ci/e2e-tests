# MVP demo test

### Prerequisites for running this test against your own cluster
1. Fork https://github.com/redhat-appstudio-qe/hacbs-test-project to your GitHub org (specified in `MY_GITHUB_ORG` env var)
2. Make sure the cluster you are about to run this test against is public (i.e. hosted on a public cloud provider)

### Description

This test simulates the "simple build -> deploy -> (failed) release" to "advanced -> build -> scan -> test -> release" user journey

For simulating the failed release we are using a custom docker-build template that contains a container image coming from ["untrusted" container image registry](https://github.com/hacbs-contract/ec-policies/blob/2d9fc8317a6349a4a9a1969f16c90dfec4448cd3/data/rule_data.yml#L9-L18). This should guarantee that the EC validation will fail and cause the failure of the release process.

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
1. Chapter 1 (simple app build, deploy, (failed) release)
   1. Setup
      1. Create a new branch in a java project on GitHub (that is used as a git source of a compoment) that will be used as a base branch for PaC pull request (so we don't pollute default branch - `main`)
      2. Create a new application, test environment
      3. Create a build pipeline selector with pipeline ref poiting to a custom build-template, that contains an image which should fail the EC validation (for more information see the [Description](#description))
      4. Create a new component that will trigger the "simple" build pipipelinerun
   2. After the component is built, check that the component is deployed (by checking the related deployment status) and check that the component route can be accessed
   3. In the meantime the release of the container image (that was built by the custom-untrusted pipelinerun) should fail for the reasons mentioned above
2. Chapter 2 (advanced build, JVM rebuild, successful release)
   1. Setup
      1. Create JBSConfig and related secret in user's (dev) namespace that will trigger jvm-build-service to deploy a jvm-cache to the dev namespace, which will be used for caching java dependencies during build
   2. Update the existing component to contain the annotation `skip-initial-checks: false` which indicates that this time we will use "advanced build" (that includes tasks for performing security checks)
   3. Verify that the initial PaC pull request was created in the component's repo (this will also trigger an advanced build pipelinerun)
   4. After merging the PR, there should be another build pipelinerun triggered in user namespace
   5. Make sure the pipelinerun completes successfully
   6. Make sure that the resulting SBOM file can be pulled from the container registry (where also the image was pushed) and it is saved in expected format
   7. Make sure JVM build service is used for rebuilding java component's dependencies and that all related dependency builds complete successfully
   8. This time we used official build-templates (that use container images from trusted registry), so the release pipeline should succeed and the release should be marked as successful
# E2E DEMOS

The e2e-demos suite contains a set of tests that covers AppStudio demos.

Steps to run 'e2e-demos-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="e2e-demos-suite"`

## Tests

The suite will cover the creation of an application in Red Hat App Studio.

Simple tests are:

* The framework creates an `Application` CR.
* Verify if the application was created successfully.
* Create a Quarkus component. [See Quarkus devfile sample](https://github.com/redhat-appstudio-qe/devfile-sample-code-with-quarkus).
* Wait for pipelinesRuns to build and push a container image to `https://quay.io/organization/redhat-appstudio-qe/quarkus:<sha1>`.
* Create a GitOps Deployment CR
* Check the GitOpsDeployment health and that the deployed image is correct
* Verify GitOpsDeployment resources in the cluster(routes, deployments, services...)
* Check GitOpsDeployment backend is working porperly
* Remove kubernetes objects created by the framework.

### Container Image source

```IN PROGRESS```

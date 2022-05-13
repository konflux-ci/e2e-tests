# HAS

Contain E2E tests related with [Hybrid Application Service Operator](https://github.com/redhat-appstudio/application-service).

Steps to run 'has-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="has-suite"`

## Tests Containers

Currently to create an application in Red Hat App Studio it is possible to create from a sample devfile or from already gitops repository created from HAS operator.

### Devfile source

Simple tests where:

* The framework create an `Application` CR.
* Verify if the application was created successfully.
* Create a Quarkus component. [See Quarkus devfile sample](https://github.com/redhat-appstudio-qe/devfile-sample-code-with-quarkus).
* Wait for pipelinesRuns to build and push a container image to `https://quay.io/organization/redhat-appstudio-qe/test-images:<sha1>`.
* Verify if HAS operator create gitops resources in the cluster(routes, deployments, services...).
* Remove kubernetes objects created by the framework.

### Container Image source

```IN PROGRESS```

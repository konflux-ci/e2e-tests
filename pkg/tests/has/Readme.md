# HAS

Contain E2E tests related with [Hybrid Application Service Operator](https://github.com/redhat-appstudio/application-service).

# Requirements to run 'has-suite' in local environment

Steps to run 'has-suite':

1) Create a fork from [infra-deployments](https://github.com/redhat-appstudio/infra-deployments) repository and clone to local machine
2) Deploy AppStudio by executing the following script: `hack/bootstrap-cluster.sh`
3) Follow instructions to deploy has controller in development mode. [Link](https://github.com/redhat-appstudio/infra-deployments#optional-configure-has-github-organization)
4) Create `redhat-appstudio-registry-pull-secret` and `redhat-appstudio-staginguser-pull-secret` secrets with your quay.io account
5) Before running the e2e-tests, it is required to set some environments:
    5.1 (Required) Set `GITHUB_TOKEN` environment value. The token value need to have the same permissions like 'has-github-token' secret. More [info](https://github.com/redhat-appstudio/application-service#creating-a-github-secret-for-has)
    5.2 (Optional) Set `GITHUB_E2E_ORGANIZATION_ENV` environment value. The github organization where the App Studio gitops repository will be created. By default: `redhat-appstudio-qe`
    5.3 (Optional) Set `QUAY_E2E_ORGANIZATION` environment value. Quay.io organization where openshift pipelines will push the quarkus component created by e2e. By default: `redhat-appstudio-qe`

6) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="has-suite"`

## Tests Containers

Currently to create an application in Red Hat App Studio it is possible to create from a sample devfile or from already gitops repository created from HAS operator.

### Devfile source

Simple tests where:

* The framework create an `Application` CR.
* Verify if the application was created successfully.
* Create a Quarkus component. [See Quarkus devfile sample](https://github.com/redhat-appstudio-qe/devfile-sample-code-with-quarkus).
* Wait for pipelinesRuns to build and push a container image to `https://quay.io/organization/redhat-appstudio-qe/quarkus:<sha1>`.
* Verify if HAS operator create gitops resources in the cluster(routes, deployments, services...).
* Remove kubernetes objects created by the framework.

### Container Image source

```IN PROGRESS```
